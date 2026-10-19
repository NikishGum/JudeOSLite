package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	errTrainerInput    = errors.New("invalid trainer input")
	errTrainerOwner    = errors.New("account requires owner")
	errTrainerVersion  = errors.New("trainer version conflict")
	errTrainerArchived = errors.New("club archived")
)

type Trainer struct {
	ID               string   `json:"id"`
	Login            string   `json:"login"`
	DisplayName      string   `json:"displayName"`
	Disabled         bool     `json:"disabled"`
	Version          int      `json:"version"`
	CanManageAccount bool     `json:"canManageAccount"`
	TrainerClubIDs   []string `json:"trainerClubIds,omitempty"` // owner roster only
}

const trainerColumns = `u.id::text,u.login,u.display_name,u.disabled_at IS NOT NULL,u.version`

func scanTrainer(row pgx.Row) (Trainer, error) {
	var u Trainer
	err := row.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Disabled, &u.Version, &u.CanManageAccount)
	return u, err
}
func trainerFail(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, errTrainerInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте имя, логин, пароль и версию записи.")
	case errors.Is(err, errTrainerOwner):
		platform.WriteError(w, 403, "account_owner_required", "Общий аккаунт и привязку существующего сотрудника меняет владелец. Можно отозвать назначение своего филиала.")
	case errors.Is(err, errTrainerVersion):
		platform.WriteError(w, 409, "version_conflict", "Запись уже изменена. Загрузите актуальный список и повторите правку.")
	case errors.Is(err, errTrainerArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Новые назначения и изменения аккаунта запрещены.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Сотрудник или филиал недоступен.")
	case errors.As(err, &p) && p.Code == "23505":
		platform.WriteError(w, 409, "login_taken", "Логин уже занят. Укажите другой; привязку существующего сотрудника выполняет владелец.")
	default:
		unavailable(w)
	}
}
func trainerDecode(w http.ResponseWriter, r *http.Request, in any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(in); err != nil || d.Decode(&struct{}{}) != io.EOF {
		trainerFail(w, errTrainerInput)
		return false
	}
	return true
}
func trainerName(value string) bool {
	if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > 200 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s *Service) trainerScope(next http.HandlerFunc, ownerOnly bool) http.Handler {
	return s.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		if ownerOnly {
			if !u.IsOwner {
				trainerFail(w, errTrainerOwner)
				return
			}
		} else {
			if _, err := s.RequireClub(r.Context(), u.ID, r.PathValue("clubID"), "administrator"); err != nil {
				trainerFail(w, err)
				return
			}
		}
		next(w, r)
	}))
}
func (s *Service) listTrainers(w http.ResponseWriter, r *http.Request) {
	owner := CurrentUser(r.Context()).IsOwner
	clubID := r.PathValue("clubID")
	if r.URL.Query().Get("available") == "true" {
		// A small selection contract for V1-06/V1-07; no credentials or foreign assignments.
		rows, err := s.pool.Query(r.Context(), `SELECT u.id::text,u.display_name FROM pride.staff_users u JOIN pride.staff_club_roles r ON r.staff_user_id=u.id JOIN pride.clubs c ON c.id=r.club_id WHERE r.club_id::text=$1 AND r.role='trainer' AND u.disabled_at IS NULL AND c.archived_at IS NULL ORDER BY u.display_name,u.id`, clubID)
		if err != nil {
			trainerFail(w, err)
			return
		}
		defer rows.Close()
		result := []map[string]string{}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				trainerFail(w, err)
				return
			}
			result = append(result, map[string]string{"id": id, "displayName": name})
		}
		if err := rows.Err(); err != nil {
			trainerFail(w, err)
			return
		}
		jsonResponse(w, 200, result)
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT `+trainerColumns+`, $2 OR (NOT u.is_owner AND (SELECT count(*) FROM pride.staff_club_roles x WHERE x.staff_user_id=u.id)=1 AND NOT EXISTS(SELECT 1 FROM pride.family_access a WHERE a.user_id=u.id)) FROM pride.staff_users u WHERE EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id::text=$1 AND r.role='trainer') AND NOT u.is_owner ORDER BY u.display_name,u.id`, clubID, owner)
	if err != nil {
		trainerFail(w, err)
		return
	}
	defer rows.Close()
	result := []Trainer{}
	for rows.Next() {
		u, err := scanTrainer(rows)
		if err != nil {
			trainerFail(w, err)
			return
		}
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, result)
}
func (s *Service) trainerRoster(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+trainerColumns+`,ARRAY(SELECT club_id::text FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.role='trainer' ORDER BY club_id) FROM pride.staff_users u WHERE NOT u.is_owner ORDER BY u.display_name,u.id`)
	if err != nil {
		trainerFail(w, err)
		return
	}
	defer rows.Close()
	result := []Trainer{}
	for rows.Next() {
		var u Trainer
		if err := rows.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Disabled, &u.Version, &u.TrainerClubIDs); err != nil {
			trainerFail(w, err)
			return
		}
		u.CanManageAccount = true
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, result)
}

// Revalidate the live actor, session and local administrator assignment under locks.
// Parent staff locks serialize role insert/revoke and account permission checks.
func (s *Service) trainerTransaction(r *http.Request, change func(pgx.Tx, string, bool) error) error {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	p := ctx.Value(contextKey{}).(principal)
	var owner bool
	if err = tx.QueryRow(ctx, `SELECT is_owner FROM pride.staff_users WHERE id=$1 AND disabled_at IS NULL FOR SHARE`, p.User.ID).Scan(&owner); err != nil {
		return err
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT true FROM pride.staff_sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now() FOR SHARE`, p.tokenHash).Scan(&live); err != nil {
		return err
	}
	clubID := r.PathValue("clubID")
	if clubID == "" {
		if !owner {
			return errTrainerOwner
		}
	} else if !owner {
		if err = tx.QueryRow(ctx, `SELECT true FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='administrator' FOR SHARE`, p.User.ID, clubID).Scan(&live); err != nil {
			return err
		}
	}
	if err = change(tx, p.User.ID, owner); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func trainerActive(ctx context.Context, tx pgx.Tx, clubID string) error {
	var archived bool
	if err := tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM pride.clubs WHERE id::text=$1 FOR SHARE`, clubID).Scan(&archived); err != nil {
		return err
	}
	if archived {
		return errTrainerArchived
	}
	return nil
}
func trainerAudit(ctx context.Context, tx pgx.Tx, actor, id, action string, clubID *string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,before_state,after_state) VALUES($1,$2,'staff_user',$3,$4,$5,$6)`, actor, clubID, id, action, b, a)
	return err
}
func trainerState(u Trainer) any { return map[string]any{"version": u.Version, "disabled": u.Disabled} }
func (s *Service) trainerHash(w http.ResponseWriter, password string) (string, bool) {
	if len(password) < 12 || len(password) > 128 {
		trainerFail(w, errTrainerInput)
		return "", false
	}
	select {
	case s.hashing <- struct{}{}:
		defer func() { <-s.hashing }()
	default:
		w.Header().Set("Retry-After", "1")
		platform.WriteError(w, 429, "too_many_attempts", "Попробуйте чуть позже.")
		return "", false
	}
	hash, err := HashPassword(password)
	if err != nil {
		trainerFail(w, err)
		return "", false
	}
	return hash, true
}
func (s *Service) createTrainer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login       string `json:"login"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	login, err := NormalizeLogin(in.Login)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if err != nil || !trainerName(in.DisplayName) {
		trainerFail(w, errTrainerInput)
		return
	}
	hash, ok := s.trainerHash(w, in.Password)
	in.Password = ""
	if !ok {
		return
	}
	var result Trainer
	err = s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		clubID := r.PathValue("clubID")
		if err := trainerActive(ctx, tx, clubID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, login, in.DisplayName, hash).Scan(&result.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, result.ID, clubID); err != nil {
			return err
		}
		var err error
		result, err = scanTrainer(tx.QueryRow(ctx, `SELECT `+trainerColumns+`,true FROM pride.staff_users u WHERE u.id=$1`, result.ID))
		if err != nil {
			return err
		}
		return trainerAudit(ctx, tx, actor, result.ID, "trainer_created", &clubID, nil, map[string]any{"version": result.Version, "assigned": true})
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 201, result)
}
func lockTrainer(ctx context.Context, tx pgx.Tx, id, clubID string, owner bool, account bool) (Trainer, error) {
	u, err := scanTrainer(tx.QueryRow(ctx, `SELECT `+trainerColumns+`,false FROM pride.staff_users u WHERE u.id::text=$1 AND NOT u.is_owner FOR UPDATE OF u`, id))
	if err != nil {
		return u, err
	}
	if clubID != "" {
		var assigned bool
		if err = tx.QueryRow(ctx, `SELECT true FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='trainer'`, u.ID, clubID).Scan(&assigned); err != nil {
			return u, err
		}
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM pride.staff_club_roles WHERE staff_user_id=$1`, u.ID).Scan(&count); err != nil {
		return u, err
	}
	var parent bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pride.family_access WHERE user_id=$1)`, u.ID).Scan(&parent); err != nil {
		return u, err
	}
	u.CanManageAccount = owner || (clubID != "" && count == 1 && !parent)
	if account && !u.CanManageAccount {
		return u, errTrainerOwner
	}
	return u, nil
}
func (s *Service) changeTrainer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login       string `json:"login"`
		DisplayName string `json:"displayName"`
		Disabled    *bool  `json:"disabled"`
		Version     int    `json:"version"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	login, err := NormalizeLogin(in.Login)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if err != nil || !trainerName(in.DisplayName) || in.Disabled == nil || in.Version < 1 {
		trainerFail(w, errTrainerInput)
		return
	}
	var result Trainer
	err = s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		clubID := r.PathValue("clubID")
		old, err := lockTrainer(ctx, tx, r.PathValue("staffID"), clubID, owner, true)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errTrainerVersion
		}
		if clubID != "" {
			if err = trainerActive(ctx, tx, clubID); err != nil {
				return err
			}
		}
		result, err = scanTrainer(tx.QueryRow(ctx, `UPDATE pride.staff_users u SET login=$2,display_name=$3,disabled_at=CASE WHEN $4 THEN coalesce(disabled_at,now()) ELSE NULL END,version=version+1,updated_at=now() WHERE u.id=$1 RETURNING `+trainerColumns+`,$5::boolean`, old.ID, login, in.DisplayName, *in.Disabled, old.CanManageAccount))
		if err != nil {
			return err
		}
		var scope *string
		if clubID != "" {
			scope = &clubID
		}
		return trainerAudit(ctx, tx, actor, old.ID, "trainer_account_updated", scope, trainerState(old), map[string]any{"version": result.Version, "disabled": result.Disabled, "loginChanged": old.Login != login, "displayNameChanged": old.DisplayName != in.DisplayName})
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, result)
}
func (s *Service) trainerPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
		Version  int    `json:"version"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		trainerFail(w, errTrainerInput)
		return
	}
	hash, ok := s.trainerHash(w, in.Password)
	in.Password = ""
	if !ok {
		return
	}
	var result Trainer
	err := s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		clubID := r.PathValue("clubID")
		old, err := lockTrainer(ctx, tx, r.PathValue("staffID"), clubID, owner, true)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errTrainerVersion
		}
		if clubID != "" {
			if err = trainerActive(ctx, tx, clubID); err != nil {
				return err
			}
		}
		result, err = scanTrainer(tx.QueryRow(ctx, `UPDATE pride.staff_users u SET password_hash=$2,version=version+1,updated_at=now() WHERE u.id=$1 RETURNING `+trainerColumns+`,$3::boolean`, old.ID, hash, old.CanManageAccount))
		if err != nil {
			return err
		}
		var scope *string
		if clubID != "" {
			scope = &clubID
		}
		return trainerAudit(ctx, tx, actor, old.ID, "trainer_password_changed", scope, trainerState(old), trainerState(result))
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, result)
}
func (s *Service) revokeTrainer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int `json:"version"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		trainerFail(w, errTrainerInput)
		return
	}
	err := s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		clubID := r.PathValue("clubID")
		old, err := lockTrainer(ctx, tx, r.PathValue("staffID"), clubID, owner, false)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errTrainerVersion
		}
		if _, err = tx.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='trainer'`, old.ID, clubID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.staff_users SET version=version+1,updated_at=now() WHERE id=$1`, old.ID); err != nil {
			return err
		}
		return trainerAudit(ctx, tx, actor, old.ID, "trainer_assignment_revoked", &clubID, map[string]any{"assigned": true, "version": old.Version}, map[string]any{"assigned": false, "version": old.Version + 1})
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}
func (s *Service) assignTrainer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClubID  string `json:"clubId"`
		Version int    `json:"version"`
	}
	if !trainerDecode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		trainerFail(w, errTrainerInput)
		return
	}
	err := s.trainerTransaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		ctx := r.Context()
		old, err := lockTrainer(ctx, tx, r.PathValue("staffID"), "", owner, false)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errTrainerVersion
		}
		if err = trainerActive(ctx, tx, in.ClubID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer') ON CONFLICT(staff_user_id,club_id,role) DO NOTHING`, old.ID, in.ClubID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.staff_users SET version=version+1,updated_at=now() WHERE id=$1`, old.ID); err != nil {
			return err
		}
		return trainerAudit(ctx, tx, actor, old.ID, "trainer_assignment_created", &in.ClubID, map[string]any{"assigned": false, "version": old.Version}, map[string]any{"assigned": true, "version": old.Version + 1})
	})
	if err != nil {
		trainerFail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// RequireTrainer is used inside future assignment transactions, after the actor's
// club authorization and lifecycle guard. A role alone never grants session writes.
func RequireTrainer(ctx context.Context, tx pgx.Tx, staffID, clubID string) error {
	var live bool
	if err := tx.QueryRow(ctx, `SELECT true FROM pride.staff_users WHERE id::text=$1 AND disabled_at IS NULL FOR SHARE`, staffID).Scan(&live); err != nil {
		return err
	}
	return tx.QueryRow(ctx, `SELECT true FROM pride.staff_club_roles WHERE staff_user_id::text=$1 AND club_id::text=$2 AND role='trainer' FOR SHARE`, staffID, clubID).Scan(&live)
}
