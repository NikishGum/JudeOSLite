package clubs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrArchived  = errors.New("club archived")
	errInput     = errors.New("invalid input")
	errConflict  = errors.New("version conflict")
	errForbidden = errors.New("owner required")
)

// RequireActive is called INSIDE the transaction that creates a new local operation
// (including future sessions). The shared lock serializes that operation with archive.
// Access must first be checked by auth.RequireClub; this is a lifecycle guard, not permission.
func RequireActive(ctx context.Context, tx pgx.Tx, clubID string) error {
	var archived bool
	if err := tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM pride.clubs WHERE id::text=$1 FOR SHARE`, clubID).Scan(&archived); err != nil {
		return err
	}
	if archived {
		return ErrArchived
	}
	return nil
}

type Club struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Timezone string `json:"timezone"`
	Archived bool   `json:"archived"`
	Version  int    `json:"version"`
}
type Staff struct {
	ID          string   `json:"id"`
	Login       string   `json:"login"`
	DisplayName string   `json:"displayName"`
	Disabled    bool     `json:"disabled"`
	ClubIDs     []string `json:"clubIds"` // administrator assignments only; trainer roles are preserved.
	Version     int      `json:"version"`
}
type Service struct {
	pool    *pgxpool.Pool
	auth    *auth.Service
	hashing chan struct{}
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service {
	return &Service{pool: pool, auth: access, hashing: make(chan struct{}, 2)}
}
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте поля формы и выбранные филиалы.")
	case errors.Is(err, errForbidden):
		platform.WriteError(w, 403, "forbidden", "Действие доступно только владельцу сети.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Запись не найдена.")
	case errors.Is(err, ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Новые назначения и операции запрещены.")
	case errors.Is(err, errConflict):
		platform.WriteError(w, 409, "version_conflict", "Запись уже изменена. Обновите список и повторите правку.")
	case errors.As(err, &p) && p.Code == "23505":
		platform.WriteError(w, 409, "login_taken", "Этот логин уже занят. Укажите другой логин.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return false
	}
	return true
}
func validText(value string, max int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= max
}
func (s *Service) owner(next http.HandlerFunc) http.Handler {
	return s.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.CurrentUser(r.Context()).IsOwner {
			fail(w, errForbidden)
			return
		}
		next(w, r)
	}))
}

// Recheck and lock the live actor in the same transaction as the audited write.
func (s *Service) transaction(r *http.Request, change func(pgx.Tx, string) error) error {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	actor := auth.CurrentUser(ctx).ID
	var enabledOwner bool
	if err = tx.QueryRow(ctx, `SELECT is_owner AND disabled_at IS NULL FROM pride.staff_users WHERE id=$1 FOR SHARE`, actor).Scan(&enabledOwner); err != nil {
		return err
	}
	if !enabledOwner {
		return errForbidden
	}
	if err = change(tx, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func audit(ctx context.Context, tx pgx.Tx, actor, entity, id, action string, clubID *string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7)`, clubID, actor, entity, id, action, b, a)
	return err
}

const clubColumns = `id::text,name,coalesce(address,''),timezone,archived_at IS NOT NULL,version`

func scanClub(row pgx.Row) (Club, error) {
	var c Club
	err := row.Scan(&c.ID, &c.Name, &c.Address, &c.Timezone, &c.Archived, &c.Version)
	return c, err
}

// Audit stores non-contact state; addresses are represented only by a change flag.
func clubState(c Club) map[string]any {
	return map[string]any{"name": c.Name, "timezone": c.Timezone, "archived": c.Archived, "version": c.Version}
}
func (s *Service) listClubs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+clubColumns+` FROM pride.clubs ORDER BY archived_at IS NOT NULL,name,id`)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	result := []Club{}
	for rows.Next() {
		c, err := scanClub(rows)
		if err != nil {
			fail(w, err)
			return
		}
		result = append(result, c)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (s *Service) getClub(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clubID")
	if _, err := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, id, "administrator", "trainer"); err != nil {
		fail(w, err)
		return
	}
	c, err := scanClub(s.pool.QueryRow(r.Context(), `SELECT `+clubColumns+` FROM pride.clubs WHERE id::text=$1`, id))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, c)
}

type clubInput struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Timezone string `json:"timezone"`
	Version  int    `json:"version"`
}

func (in *clubInput) validate() bool {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	in.Timezone = strings.TrimSpace(in.Timezone)
	if !validText(in.Name, 200) || utf8.RuneCountInString(in.Address) > 500 || strings.ContainsRune(in.Address, 0) || in.Timezone == "Local" || in.Timezone == "" {
		return false
	}
	_, err := time.LoadLocation(in.Timezone)
	return err == nil
}
func (s *Service) saveClub(w http.ResponseWriter, r *http.Request) {
	var in clubInput
	if !decode(w, r, &in) {
		return
	}
	if !in.validate() {
		fail(w, errInput)
		return
	}
	var result Club
	err := s.transaction(r, func(tx pgx.Tx, actor string) error {
		ctx := r.Context()
		id := r.PathValue("clubID")
		var before any
		addressChanged := false
		action := "created"
		if id == "" {
			if in.Version != 0 {
				return errInput
			}
			var err error
			result, err = scanClub(tx.QueryRow(ctx, `INSERT INTO pride.clubs(name,address,timezone) VALUES($1,nullif($2,''),$3) RETURNING `+clubColumns, in.Name, in.Address, in.Timezone))
			if err != nil {
				return err
			}
		} else {
			old, err := scanClub(tx.QueryRow(ctx, `SELECT `+clubColumns+` FROM pride.clubs WHERE id::text=$1 FOR UPDATE`, id))
			if err != nil {
				return err
			}
			if old.Version != in.Version {
				return errConflict
			}
			before = clubState(old)
			addressChanged = old.Address != in.Address
			action = "updated"
			result, err = scanClub(tx.QueryRow(ctx, `UPDATE pride.clubs SET name=$2,address=nullif($3,''),timezone=$4,updated_at=now(),version=version+1 WHERE id::text=$1 RETURNING `+clubColumns, id, in.Name, in.Address, in.Timezone))
			if err != nil {
				return err
			}
		}
		after := clubState(result)
		if action == "updated" {
			after["addressChanged"] = addressChanged
		}
		return audit(ctx, tx, actor, "club", result.ID, action, &result.ID, before, after)
	})
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	respond(w, status, result)
}
func (s *Service) archiveClub(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	var result Club
	err := s.transaction(r, func(tx pgx.Tx, actor string) error {
		ctx := r.Context()
		old, err := scanClub(tx.QueryRow(ctx, `SELECT `+clubColumns+` FROM pride.clubs WHERE id::text=$1 FOR UPDATE`, r.PathValue("clubID")))
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errConflict
		}
		if old.Archived {
			result = old
			return nil
		}
		result, err = scanClub(tx.QueryRow(ctx, `UPDATE pride.clubs SET archived_at=now(),updated_at=now(),version=version+1 WHERE id=$1 RETURNING `+clubColumns, old.ID))
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "club", old.ID, "archived", &old.ID, clubState(old), clubState(result))
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}

const staffColumns = `u.id::text,u.login,u.display_name,u.disabled_at IS NOT NULL,u.version,
 ARRAY(SELECT r.club_id::text FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.role='administrator' ORDER BY r.club_id)`

func scanStaff(row pgx.Row) (Staff, error) {
	var u Staff
	err := row.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Disabled, &u.Version, &u.ClubIDs)
	return u, err
}
func staffState(u Staff) any {
	return map[string]any{"disabled": u.Disabled, "clubIds": u.ClubIDs, "version": u.Version}
}
func (s *Service) listStaff(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+staffColumns+` FROM pride.staff_users u WHERE NOT u.is_owner AND `+auth.StaffRegistryPredicate+` ORDER BY u.display_name,u.id`)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	result := []Staff{}
	for rows.Next() {
		u, err := scanStaff(rows)
		if err != nil {
			fail(w, err)
			return
		}
		result = append(result, u)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}

type staffInput struct {
	Login       string   `json:"login"`
	DisplayName string   `json:"displayName"`
	Password    string   `json:"password"`
	ClubIDs     []string `json:"clubIds"`
}

func normalizeClubs(ids []string) ([]string, error) {
	if ids == nil || len(ids) > 100 {
		return nil, errInput
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range ids {
		var id pgtype.UUID
		if err := id.Scan(value); err != nil || !id.Valid {
			return nil, errInput
		}
		value = strings.ToLower(value)
		if seen[value] {
			return nil, errInput
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}
func validateClubs(ctx context.Context, tx pgx.Tx, ids, old []string) error {
	// Lock in stable order. Existing archived assignments can be retained or revoked;
	// an archived club cannot receive a new assignment.
	for _, id := range ids {
		err := RequireActive(ctx, tx, id)
		if errors.Is(err, ErrArchived) {
			keep := false
			for _, existing := range old {
				if id == existing {
					keep = true
				}
			}
			if keep {
				continue
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return errInput
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func setAssignments(ctx context.Context, tx pgx.Tx, id string, clubIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator' AND NOT (club_id::text=ANY($2::text[]))`, id, clubIDs); err != nil {
		return err
	}
	for _, clubID := range clubIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator') ON CONFLICT(staff_user_id,club_id,role) DO NOTHING`, id, clubID); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) createStaff(w http.ResponseWriter, r *http.Request) {
	var in staffInput
	if !decode(w, r, &in) {
		return
	}
	login, err := auth.NormalizeLogin(in.Login)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	ids, idErr := normalizeClubs(in.ClubIDs)
	if err != nil || idErr != nil || len(ids) == 0 || !validText(in.DisplayName, 200) || len(in.Password) < 12 || len(in.Password) > 128 {
		fail(w, errInput)
		return
	}
	select {
	case s.hashing <- struct{}{}:
		defer func() { <-s.hashing }()
	default:
		w.Header().Set("Retry-After", "1")
		platform.WriteError(w, 429, "too_many_attempts", "Попробуйте создать аккаунт чуть позже.")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	in.Password = ""
	if err != nil {
		fail(w, err)
		return
	}
	var result Staff
	err = s.transaction(r, func(tx pgx.Tx, actor string) error {
		ctx := r.Context()
		if err := validateClubs(ctx, tx, ids, nil); err != nil {
			return err
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, login, in.DisplayName, hash).Scan(&id); err != nil {
			return err
		}
		if err := setAssignments(ctx, tx, id, ids); err != nil {
			return err
		}
		var err error
		result, err = scanStaff(tx.QueryRow(ctx, `SELECT `+staffColumns+` FROM pride.staff_users u WHERE u.id=$1`, id))
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "staff_user", id, "administrator_created", nil, nil, staffState(result))
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, result)
}
func (s *Service) updateStaff(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName string   `json:"displayName"`
		ClubIDs     []string `json:"clubIds"`
		Disabled    *bool    `json:"disabled"`
		Version     int      `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	ids, err := normalizeClubs(in.ClubIDs)
	if err != nil || !validText(in.DisplayName, 200) || in.Disabled == nil {
		fail(w, errInput)
		return
	}
	var result Staff
	err = s.transaction(r, func(tx pgx.Tx, actor string) error {
		ctx := r.Context()
		old, err := scanStaff(tx.QueryRow(ctx, `SELECT `+staffColumns+` FROM pride.staff_users u WHERE u.id::text=$1 AND NOT u.is_owner FOR UPDATE OF u`, r.PathValue("staffID")))
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errConflict
		}
		if err = validateClubs(ctx, tx, ids, old.ClubIDs); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.staff_users SET display_name=$2,disabled_at=CASE WHEN $3 THEN coalesce(disabled_at,now()) ELSE NULL END,updated_at=now(),version=version+1 WHERE id=$1`, old.ID, in.DisplayName, *in.Disabled); err != nil {
			return err
		}
		if err = setAssignments(ctx, tx, old.ID, ids); err != nil {
			return err
		}
		result, err = scanStaff(tx.QueryRow(ctx, `SELECT `+staffColumns+` FROM pride.staff_users u WHERE u.id=$1`, old.ID))
		if err != nil {
			return err
		}
		after := map[string]any{"disabled": result.Disabled, "clubIds": result.ClubIDs, "version": result.Version, "displayNameChanged": old.DisplayName != result.DisplayName}
		return audit(ctx, tx, actor, "staff_user", old.ID, "administration_updated", nil, staffState(old), after)
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/clubs", s.owner(s.listClubs))
	mux.Handle("POST /api/clubs", s.owner(s.saveClub))
	mux.Handle("GET /api/clubs/{clubID}", s.auth.Require(http.HandlerFunc(s.getClub)))
	mux.Handle("PUT /api/clubs/{clubID}", s.owner(s.saveClub))
	mux.Handle("POST /api/clubs/{clubID}/archive", s.owner(s.archiveClub))
	mux.Handle("GET /api/staff", s.owner(s.listStaff))
	mux.Handle("POST /api/staff", s.owner(s.createStaff))
	mux.Handle("PUT /api/staff/{staffID}", s.owner(s.updateStaff))
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/clubs") || strings.HasPrefix(r.URL.Path, "/api/staff") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
