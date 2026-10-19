// Package families implements explicit, club-local family access using the
// existing staff identity and session boundary. No name/phone matching occurs.
package families

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errInput = errors.New("invalid family input")
var errConflict = errors.New("family version or membership conflict")
var errSharedAccount = errors.New("shared account requires owner")

type Service struct {
	pool    *pgxpool.Pool
	auth    *auth.Service
	hashing chan struct{}
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service {
	return &Service{pool, access, make(chan struct{}, 2)}
}

type MemberSummary struct {
	ID          string `json:"id"`
	ClubID      string `json:"clubId"`
	FullName    string `json:"fullName"`
	BirthDate   string `json:"birthDate"`
	GroupName   string `json:"groupName"`
	TrainerName string `json:"trainerName"`
	Archived    bool   `json:"archived"`
}
type ParentAccount struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}
type Family struct {
	ID          string          `json:"id"`
	ClubID      string          `json:"clubId"`
	Name        string          `json:"name"`
	ContactName string          `json:"contactName"`
	Phone       string          `json:"phone"`
	Email       string          `json:"email"`
	Version     int             `json:"version"`
	Children    []MemberSummary `json:"children"`
	Accounts    []ParentAccount `json:"accounts"`
}
type familyInput struct {
	Name          string   `json:"name"`
	ContactName   *string  `json:"contactName"`
	Phone         *string  `json:"phone"`
	Email         *string  `json:"email"`
	MembershipIDs []string `json:"membershipIds"`
	Version       *int     `json:"version"`
}

func text(value string, max int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > max {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (in *familyInput) valid() bool {
	if in.ContactName == nil || in.Phone == nil || in.Email == nil {
		return false
	}
	in.Name = strings.TrimSpace(in.Name)
	*in.ContactName = strings.TrimSpace(*in.ContactName)
	*in.Phone = strings.TrimSpace(*in.Phone)
	*in.Email = strings.TrimSpace(*in.Email)
	if in.Version == nil || in.Name == "" || !text(in.Name, 200) || !text(*in.ContactName, 200) || !text(*in.Phone, 80) || !text(*in.Email, 254) || in.MembershipIDs == nil || len(in.MembershipIDs) > 100 {
		return false
	}

	seen := map[string]bool{}
	for i, v := range in.MembershipIDs {
		var id pgtype.UUID
		if id.Scan(v) != nil || !id.Valid {
			return false
		}
		v = strings.ToLower(v)
		if seen[v] {
			return false
		}
		seen[v] = true
		in.MembershipIDs[i] = v
	}
	sort.Strings(in.MembershipIDs)
	return true
}
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте поля, состав семьи и версию записи.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Семья, карточка или филиал недоступны.")
	case errors.Is(err, errSharedAccount):
		platform.WriteError(w, 403, "forbidden", "Пароль общего аккаунта меняет владелец сети.")
	case errors.Is(err, clubs.ErrArchived):
		platform.WriteError(w, 409, "version_conflict", "Филиал в архиве. Изменения запрещены.")
	case errors.Is(err, errConflict), errors.As(err, &p) && (p.Code == "23505" || p.Code == "40001" || p.Code == "40P01"):
		platform.WriteError(w, 409, "version_conflict", "Запись изменена, логин занят или участник уже связан с семьёй. Обновите данные.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := strings.Split(r.Header.Get("Content-Type"), ";")[0]; ct != "application/json" {
		fail(w, errInput)
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return false
	}
	return true
}
func (s *Service) transaction(r *http.Request, manager, active bool, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if manager {
		_, err = auth.LockClubAdministrator(r.Context(), tx, r.PathValue("clubID"))
	} else {
		_, err = auth.LockSession(r.Context(), tx)
	}
	if err != nil {
		return err
	}
	if active {
		if err = clubs.RequireActive(r.Context(), tx, r.PathValue("clubID")); err != nil {
			return err
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}
func (s *Service) protected(next http.HandlerFunc) http.Handler { return s.auth.Require(next) }
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/clubs/{clubID}/families", s.protected(s.list))
	mux.Handle("POST /api/clubs/{clubID}/families", s.protected(s.save))
	mux.Handle("GET /api/clubs/{clubID}/families/{familyID}", s.protected(s.get))
	mux.Handle("PUT /api/clubs/{clubID}/families/{familyID}", s.protected(s.save))
	mux.Handle("POST /api/clubs/{clubID}/families/{familyID}/accounts", s.protected(s.account))
	mux.Handle("POST /api/clubs/{clubID}/families/{familyID}/accounts/{userID}/reset-password", s.protected(s.account))
	mux.Handle("POST /api/clubs/{clubID}/families/{familyID}/accounts/{userID}/revoke", s.protected(s.account))
	mux.Handle("GET /api/portal/families", s.protected(s.list))
	mux.Handle("GET /api/clubs/{clubID}/member-profiles/{membershipID}", s.protected(s.profile))
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/families") || strings.Contains(r.URL.Path, "/member-profiles/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
func pagination(r *http.Request) (int, int, error) {
	page, limit := 0, 25
	var err error
	if raw, ok := r.URL.Query()["page"]; ok {
		page, err = strconv.Atoi(raw[0])
		if err != nil {
			return 0, 0, errInput
		}
	}
	if raw, ok := r.URL.Query()["limit"]; ok {
		limit, err = strconv.Atoi(raw[0])
		if err != nil {
			return 0, 0, errInput
		}
	}
	if page < 0 || page > 1000000 || limit < 1 || limit > 100 || !text(r.URL.Query().Get("q"), 200) {
		return 0, 0, errInput
	}
	return page, limit, nil
}

const familyCols = `f.id::text,f.club_id::text,f.name,f.contact_name,f.phone,f.email,f.version`

func scanFamily(row pgx.Row) (Family, error) {
	f := Family{Children: []MemberSummary{}, Accounts: []ParentAccount{}}
	err := row.Scan(&f.ID, &f.ClubID, &f.Name, &f.ContactName, &f.Phone, &f.Email, &f.Version)
	return f, err
}
func loadFamily(ctx context.Context, tx pgx.Tx, club, id string, lock bool) (Family, error) {
	query := `SELECT ` + familyCols + ` FROM pride.families f WHERE f.id::text=$1 AND f.club_id::text=$2`
	if lock {
		query += ` FOR UPDATE`
	} else {
		query += ` FOR SHARE`
	}
	f, err := scanFamily(tx.QueryRow(ctx, query, id, club))
	if err != nil {
		return f, err
	}
	return populate(ctx, tx, f)
}
func populate(ctx context.Context, tx pgx.Tx, f Family) (Family, error) {
	rows, err := tx.Query(ctx, `SELECT `+memberCols+` FROM pride.family_children fc JOIN pride.club_memberships m ON m.id=fc.membership_id JOIN pride.participants p ON p.id=m.participant_id JOIN pride.clubs c ON c.id=m.club_id `+currentGroup+` WHERE fc.family_id=$1 ORDER BY lower(p.full_name),m.id`, f.ID)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var m MemberSummary
		if err = scanMember(rows, &m); err != nil {
			break
		}
		f.Children = append(f.Children, m)
	}
	rows.Close()
	if err != nil {
		return f, err
	}
	if err = rows.Err(); err != nil {
		return f, err
	}
	rows, err = tx.Query(ctx, `SELECT u.id::text,u.login,u.display_name FROM pride.family_access a JOIN pride.staff_users u ON u.id=a.user_id WHERE a.family_id=$1 ORDER BY u.login,u.id`, f.ID)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	for rows.Next() {
		var a ParentAccount
		if err = rows.Scan(&a.ID, &a.Login, &a.DisplayName); err != nil {
			return f, err
		}
		f.Accounts = append(f.Accounts, a)
	}
	return f, rows.Err()
}
func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	var result Family
	err := s.transaction(r, true, false, func(tx pgx.Tx) error {
		var e error
		result, e = loadFamily(r.Context(), tx, r.PathValue("clubID"), r.PathValue("familyID"), false)
		return e
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	page, limit, err := pagination(r)
	if err != nil {
		fail(w, err)
		return
	}
	portal := r.URL.Path == "/api/portal/families"
	if !portal {
		var id pgtype.UUID
		if id.Scan(r.PathValue("clubID")) != nil || !id.Valid {
			fail(w, pgx.ErrNoRows)
			return
		}
	}
	result := []Family{}
	total := 0
	err = s.transaction(r, !portal, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		where := `f.club_id=$1::uuid`
		args := []any{r.PathValue("clubID"), r.URL.Query().Get("q")}
		if portal {
			where = `c.archived_at IS NULL AND EXISTS(SELECT 1 FROM pride.family_access a WHERE a.family_id=f.id AND a.user_id=$1::uuid)`
			args[0] = auth.CurrentUser(ctx).ID
		}
		where += ` AND ($2='' OR strpos(lower(f.name||' '||f.contact_name||' '||f.phone||' '||f.email),lower($2))>0)`
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM pride.families f JOIN pride.clubs c ON c.id=f.club_id WHERE `+where, args...).Scan(&total); err != nil {
			return err
		}
		args = append(args, limit, page*limit)
		rows, err := tx.Query(ctx, `SELECT `+familyCols+` FROM pride.families f JOIN pride.clubs c ON c.id=f.club_id WHERE `+where+` ORDER BY lower(f.name),f.id LIMIT $3 OFFSET $4 FOR SHARE OF f`, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			f, e := scanFamily(rows)
			if e != nil {
				rows.Close()
				return e
			}
			result = append(result, f)
		}
		rows.Close()
		if rows.Err() != nil {
			return rows.Err()
		}
		for i, f := range result {
			if portal {
				var ok bool
				if err = tx.QueryRow(ctx, `SELECT true FROM pride.family_access WHERE family_id=$1 AND user_id=$2 FOR SHARE`, f.ID, auth.CurrentUser(ctx).ID).Scan(&ok); err != nil {
					return err
				}
			}
			result[i], err = populate(ctx, tx, f)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]any{"items": result, "total": total, "hasMore": page*limit+len(result) < total})
}
func (s *Service) save(w http.ResponseWriter, r *http.Request) {
	var in familyInput
	if !decode(w, r, &in) {
		return
	}
	creating := r.Method == "POST"
	if !in.valid() || (creating && *in.Version != 0) || (!creating && *in.Version < 1) {
		fail(w, errInput)
		return
	}
	var result Family
	err := s.transaction(r, true, true, func(tx pgx.Tx) error {
		ctx := r.Context()
		club, id := r.PathValue("clubID"), r.PathValue("familyID")
		action := "updated"
		if creating {
			action = "created"
			if err := tx.QueryRow(ctx, `INSERT INTO pride.families(club_id,name,contact_name,phone,email) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, club, in.Name, *in.ContactName, *in.Phone, *in.Email).Scan(&id); err != nil {
				return err
			}
		} else {
			old, err := loadFamily(ctx, tx, club, id, true)
			if err != nil {
				return err
			}
			if old.Version != *in.Version {
				return errConflict
			}
			if _, err = tx.Exec(ctx, `UPDATE pride.families SET name=$2,contact_name=$3,phone=$4,email=$5,version=version+1,updated_at=now() WHERE id=$1`, id, in.Name, *in.ContactName, *in.Phone, *in.Email); err != nil {
				return err
			}
		}
		// The unique membership key resolves concurrent claims, even across families.
		// Memberships are explicit and club-local; archived children may remain linked.
		for _, member := range in.MembershipIDs {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT true FROM pride.club_memberships WHERE id::text=$1 AND club_id::text=$2 FOR KEY SHARE`, member, club).Scan(&ok); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM pride.family_children WHERE family_id=$1 AND NOT (membership_id::text=ANY($2::text[]))`, id, in.MembershipIDs); err != nil {
			return err
		}
		for _, member := range in.MembershipIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3) ON CONFLICT(family_id,membership_id) DO NOTHING`, id, club, member); err != nil {
				return err
			}
		}
		var err error
		result, err = loadFamily(ctx, tx, club, id, false)
		if err != nil {
			return err
		}
		return audit(ctx, tx, auth.CurrentUser(ctx).ID, result, action, "")
	})
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	respond(w, status, result)
}

// Audit contains identifiers and versions only; family contacts, children names,
// login, password/hash and medical information never enter the event payload.
func audit(ctx context.Context, tx pgx.Tx, actor string, f Family, action, user string) error {
	state, _ := json.Marshal(map[string]any{"version": f.Version, "accountId": user})
	_, err := tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,$4,$5)`, f.ClubID, actor, f.ID, "family_"+action, state)
	return err
}
