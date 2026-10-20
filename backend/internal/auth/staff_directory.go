package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// StaffRegistryPredicate uses the alias u. A shared login table is not a staff
// registry: parent-only identities have neither a role nor a staffing event.
// Staffing events survive removal of the final role, keeping former staff visible.
const StaffRegistryPredicate = `(EXISTS(SELECT 1 FROM pride.staff_club_roles sr WHERE sr.staff_user_id=u.id)
 OR EXISTS(SELECT 1 FROM pride.audit_events se WHERE se.entity_type='staff_user' AND se.entity_id=u.id
 AND se.action IN ('administrator_created','administration_updated','trainer_created',
 'trainer_assignment_created','trainer_assignment_revoked','administrator_assignment_created','administrator_assignment_revoked')))`

type StaffAssignment struct {
	ClubID    string `json:"clubId"`
	ClubName  string `json:"clubName"`
	Role      string `json:"role"`
	Archived  bool   `json:"archived"`
	CanRevoke bool   `json:"canRevoke"`
}

type StaffCard struct {
	Trainer
	Assignments []StaffAssignment `json:"assignments"`
}

type StaffDirectoryClub struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type StaffDirectory struct {
	Staff             []StaffCard          `json:"staff"`
	Clubs             []StaffDirectoryClub `json:"clubs"`
	CanAssignExisting bool                 `json:"canAssignExisting"`
}

func (s *Service) staffDirectory(w http.ResponseWriter, r *http.Request) {
	s.readStaffDirectory(w, r, false)
}

// An explicit owner search can find a parent-only login without treating that
// identity as staff or exposing its family relationships.
func (s *Service) staffCandidates(w http.ResponseWriter, r *http.Request) {
	s.readStaffDirectory(w, r, true)
}

func (s *Service) readStaffDirectory(w http.ResponseWriter, r *http.Request, candidates bool) {
	ctx := r.Context()
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if candidates && (utf8.RuneCountInString(query) < 3 || utf8.RuneCountInString(query) > 100) {
		trainerFail(w, errTrainerInput)
		return
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		trainerFail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	p := ctx.Value(contextKey{}).(principal)
	var owner bool
	// Recheck actor/session inside the same snapshot as clubs, versions and roles.
	err = tx.QueryRow(ctx, `SELECT u.is_owner FROM pride.staff_users u JOIN pride.staff_sessions ss ON ss.staff_user_id=u.id
 WHERE u.id=$1 AND u.disabled_at IS NULL AND ss.token_hash=$2 AND ss.revoked_at IS NULL AND ss.expires_at>now()`, p.User.ID, p.tokenHash).Scan(&owner)
	if err != nil {
		trainerFail(w, err)
		return
	}
	if candidates && !owner {
		trainerFail(w, errTrainerOwner)
		return
	}
	result := StaffDirectory{Staff: []StaffCard{}, Clubs: []StaffDirectoryClub{}, CanAssignExisting: owner}
	rows, err := tx.Query(ctx, `SELECT c.id::text,c.name,c.archived_at IS NOT NULL FROM pride.clubs c
 WHERE $2 OR EXISTS(SELECT 1 FROM pride.staff_club_roles ar WHERE ar.staff_user_id=$1 AND ar.club_id=c.id AND ar.role='administrator')
 ORDER BY c.name,c.id`, p.User.ID, owner)
	if err != nil {
		trainerFail(w, err)
		return
	}
	for rows.Next() {
		var c StaffDirectoryClub
		if err = rows.Scan(&c.ID, &c.Name, &c.Archived); err != nil {
			break
		}
		result.Clubs = append(result.Clubs, c)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		trainerFail(w, err)
		return
	}
	if !owner && len(result.Clubs) == 0 {
		platform.WriteError(w, 403, "forbidden", "Управление сотрудниками недоступно.")
		return
	}
	clubIDs := make([]string, 0, len(result.Clubs))
	for _, c := range result.Clubs {
		clubIDs = append(clubIDs, c.ID)
	}
	rows, err = tx.Query(ctx, `SELECT `+trainerColumns+`,
 $1 OR ((SELECT count(*) FROM pride.staff_club_roles x WHERE x.staff_user_id=u.id)=1
 AND NOT EXISTS(SELECT 1 FROM pride.family_access fa WHERE fa.user_id=u.id)
 AND EXISTS(SELECT 1 FROM pride.staff_club_roles x JOIN pride.clubs c ON c.id=x.club_id
 WHERE x.staff_user_id=u.id AND x.role='trainer' AND x.club_id::text=ANY($2::text[]) AND c.archived_at IS NULL)),
 coalesce((SELECT jsonb_agg(jsonb_build_object('clubId',c.id::text,'clubName',c.name,'role',x.role,
 'archived',c.archived_at IS NOT NULL,'canRevoke',true) ORDER BY c.name,c.id,x.role)
 FROM pride.staff_club_roles x JOIN pride.clubs c ON c.id=x.club_id WHERE x.staff_user_id=u.id
 AND ($1 OR (x.role='trainer' AND x.club_id::text=ANY($2::text[])))),'[]'::jsonb)
 FROM pride.staff_users u WHERE NOT u.is_owner AND ($3 OR `+StaffRegistryPredicate+`)
 AND (NOT $3 OR u.login ILIKE $4 ESCAPE '\' OR u.display_name ILIKE $4 ESCAPE '\')
 AND ($1 OR EXISTS(SELECT 1 FROM pride.staff_club_roles x WHERE x.staff_user_id=u.id AND x.role='trainer' AND x.club_id::text=ANY($2::text[])))
 ORDER BY u.display_name,u.id LIMIT CASE WHEN $3 THEN 30 ELSE NULL END`, owner, clubIDs, candidates,
		"%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)+"%")
	if err != nil {
		trainerFail(w, err)
		return
	}
	for rows.Next() {
		var card StaffCard
		var assignments []byte
		err = rows.Scan(&card.ID, &card.Login, &card.DisplayName, &card.Disabled, &card.Version, &card.CanManageAccount, &assignments)
		if err == nil {
			err = json.Unmarshal(assignments, &card.Assignments)
		}
		if err != nil {
			break
		}
		result.Staff = append(result.Staff, card)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		trainerFail(w, err)
		return
	}
	if candidates {
		jsonResponse(w, 200, result.Staff)
	} else {
		jsonResponse(w, 200, result)
	}
}

// Unlike the legacy administrator bulk replacement, this changes one role in
// one club. Trainer assignments and every other administrator role survive.
func (s *Service) administratorAssignment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClubID  string `json:"clubId"`
		Version int    `json:"version"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	var clubID pgtype.UUID
	if in.Version < 1 || clubID.Scan(in.ClubID) != nil || !clubID.Valid {
		trainerFail(w, errTrainerInput)
		return
	}
	revoke := r.PathValue("operation") == "revoke"
	err := s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		old, err := lockTrainer(ctx, tx, r.PathValue("staffID"), "", owner, false)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errTrainerVersion
		}
		var changed int64
		if revoke {
			tag, err := tx.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='administrator'`, old.ID, in.ClubID)
			if err != nil {
				return err
			}
			changed = tag.RowsAffected()
		} else {
			if err = trainerActive(ctx, tx, in.ClubID); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator') ON CONFLICT DO NOTHING`, old.ID, in.ClubID)
			if err != nil {
				return err
			}
			changed = tag.RowsAffected()
		}
		if changed == 0 {
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.staff_users SET version=version+1,updated_at=now() WHERE id=$1`, old.ID); err != nil {
			return err
		}
		action := "administrator_assignment_created"
		if revoke {
			action = "administrator_assignment_revoked"
		}
		return trainerAudit(ctx, tx, actor, old.ID, action, &in.ClubID,
			map[string]any{"assigned": revoke, "version": old.Version},
			map[string]any{"assigned": !revoke, "version": old.Version + 1})
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}
