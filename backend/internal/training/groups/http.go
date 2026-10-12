package groups

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInput = errors.New("invalid group input")
var ErrConflict = errors.New("version conflict")
var ErrArchived = errors.New("group or card archived")
var ErrOverlap = errors.New("overlapping intervals")

type Group struct {
	ID       string `json:"id"`
	ClubID   string `json:"clubId"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Version  int    `json:"version"`
}
type Interval struct {
	ID            string  `json:"id"`
	ClubID        string  `json:"clubId"`
	GroupID       string  `json:"groupId"`
	MembershipID  string  `json:"membershipId"`
	GroupName     string  `json:"groupName"`
	FullName      string  `json:"fullName"`
	BirthDate     string  `json:"birthDate"`
	CardArchived  bool    `json:"cardArchived"`
	GroupArchived bool    `json:"groupArchived"`
	From          string  `json:"from"`
	Until         *string `json:"until"`
	Version       int     `json:"version"`
}
type InitialGroup struct {
	GroupID string `json:"groupId"`
	From    string `json:"from"`
}
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }
func ValidDate(value string) bool {
	d, e := time.Parse("2006-01-02", value)
	return e == nil && len(value) == 10 && d.Year() >= 1 && d.Format("2006-01-02") == value
}
func validName(v string) bool {
	if !utf8.ValidString(v) || v == "" || utf8.RuneCountInString(v) > 200 {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func NormalizeError(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23P01":
			return ErrOverlap
		case "23514":
			return ErrInput
		case "23503", "22P02":
			return pgx.ErrNoRows
		}
	}
	return err
}
func Fail(w http.ResponseWriter, err error) {
	err = NormalizeError(err)
	switch {
	case errors.Is(err, ErrInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте название, полные даты и версии. Конец периода позже начала; начало не раньше вступления в филиал.")
	case errors.Is(err, ErrConflict):
		platform.WriteError(w, 409, "version_conflict", "Запись уже изменена. Загрузите актуальные данные и повторите действие.")
	case errors.Is(err, ErrOverlap):
		platform.WriteError(w, 409, "interval_overlap", "На эти даты участник уже состоит в группе. Используйте перевод или выберите свободный период.")
	case errors.Is(err, ErrArchived):
		platform.WriteError(w, 409, "group_archived", "Группа или карточка в архиве. Новое включение недоступно.")
	case errors.Is(err, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Изменения недоступны.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Группа, участник или филиал недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		Fail(w, ErrInput)
		return false
	}
	return true
}
func (s *Service) access(fn http.HandlerFunc) http.Handler {
	return s.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, r.PathValue("clubID"), "administrator"); e != nil {
			Fail(w, e)
			return
		}
		fn(w, r)
	}))
}
func (s *Service) transaction(r *http.Request, fn func(pgx.Tx) error) error {
	ctx := r.Context()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = auth.LockClubAdministrator(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	if e = clubs.RequireActive(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func Audit(ctx context.Context, tx pgx.Tx, actor, club, id, kind, action string, before, after any) error {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	_, e := tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7)`, club, actor, kind, id, action, b, a)
	return e
}
func groupState(g Group) any { return map[string]any{"version": g.Version, "archived": g.Archived} }
func intervalState(i Interval) any {
	return map[string]any{"groupId": i.GroupID, "membershipId": i.MembershipID, "from": i.From, "until": i.Until, "version": i.Version}
}
func scanGroup(row pgx.Row) (Group, error) {
	var g Group
	e := row.Scan(&g.ID, &g.ClubID, &g.Name, &g.Archived, &g.Version)
	return g, e
}

const groupColumns = `id::text,club_id::text,name,archived_at IS NOT NULL,version`

func ReadGroup(ctx context.Context, tx pgx.Tx, club, id string, active bool) (Group, error) {
	g, e := scanGroup(tx.QueryRow(ctx, `SELECT `+groupColumns+` FROM pride.training_groups WHERE club_id::text=$1 AND id::text=$2 FOR SHARE`, club, id))
	if e == nil && active && g.Archived {
		return g, ErrArchived
	}
	return g, e
}

// Lock identity before card, matching registry writes. The same parent serializes
// concurrent additions/transfers, card archive and direct interval writes.
func lockCard(ctx context.Context, tx pgx.Tx, club, id string) (bool, error) {
	var person, locked string
	var archived bool
	if e := tx.QueryRow(ctx, `SELECT participant_id::text FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2`, club, id).Scan(&person); e != nil {
		return false, e
	}
	if e := tx.QueryRow(ctx, `SELECT id::text FROM pride.participants WHERE id=$1 FOR UPDATE`, person).Scan(&locked); e != nil {
		return false, e
	}
	e := tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2 FOR UPDATE`, club, id).Scan(&archived)
	return archived, e
}

const intervalColumns = `m.id::text,m.club_id::text,m.group_id::text,m.club_membership_id::text,g.name,p.full_name,to_char(p.birth_date,'YYYY-MM-DD'),c.archived_at IS NOT NULL,g.archived_at IS NOT NULL,to_char(m.valid_from,'YYYY-MM-DD'),to_char(m.valid_until,'YYYY-MM-DD'),m.version`
const intervalJoins = ` FROM pride.group_memberships m JOIN pride.training_groups g ON g.id=m.group_id JOIN pride.club_memberships c ON c.id=m.club_membership_id JOIN pride.participants p ON p.id=c.participant_id `

func scanInterval(row pgx.Row) (Interval, error) {
	var i Interval
	e := row.Scan(&i.ID, &i.ClubID, &i.GroupID, &i.MembershipID, &i.GroupName, &i.FullName, &i.BirthDate, &i.CardArchived, &i.GroupArchived, &i.From, &i.Until, &i.Version)
	return i, e
}
func readInterval(ctx context.Context, tx pgx.Tx, club, id string) (Interval, error) {
	return scanInterval(tx.QueryRow(ctx, `SELECT `+intervalColumns+intervalJoins+`WHERE m.club_id::text=$1 AND m.id::text=$2`, club, id))
}

// AddInitial runs in the registry's existing transaction; no separate HTTP call.
func AddInitial(ctx context.Context, tx pgx.Tx, actor, club, member string, in InitialGroup) error {
	if !ValidDate(in.From) {
		return ErrInput
	}
	if _, e := ReadGroup(ctx, tx, club, in.GroupID, true); e != nil {
		return e
	}
	_, e := insert(ctx, tx, actor, club, member, in.GroupID, in.From, nil)
	return e
}
func insert(ctx context.Context, tx pgx.Tx, actor, club, member, group, from string, until *string) (Interval, error) {
	var id string
	e := tx.QueryRow(ctx, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from,valid_until) VALUES($1,$2,$3,$4::date,$5::date) RETURNING id::text`, club, group, member, from, until).Scan(&id)
	if e != nil {
		return Interval{}, e
	}
	i, e := readInterval(ctx, tx, club, id)
	if e != nil {
		return i, e
	}
	return i, Audit(ctx, tx, actor, club, id, "group_membership", "added", nil, intervalState(i))
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("state")
	if filter == "" {
		filter = "active"
	}
	if filter != "active" && filter != "archived" && filter != "all" {
		Fail(w, ErrInput)
		return
	}
	rows, e := s.pool.Query(r.Context(), `SELECT `+groupColumns+` FROM pride.training_groups WHERE club_id::text=$1 AND ($2='all' OR (archived_at IS NOT NULL)=($2='archived')) ORDER BY lower(name),id`, r.PathValue("clubID"), filter)
	if e != nil {
		Fail(w, e)
		return
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		g, e := scanGroup(rows)
		if e != nil {
			Fail(w, e)
			return
		}
		out = append(out, g)
	}
	if e = rows.Err(); e != nil {
		Fail(w, e)
		return
	}
	respond(w, 200, out)
}
func (s *Service) save(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	creating := r.Method == "POST"
	if !validName(in.Name) || (creating && in.Version != 0) || (!creating && in.Version < 1) {
		Fail(w, ErrInput)
		return
	}
	var result Group
	e := s.transaction(r, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		actor := auth.CurrentUser(ctx).ID
		var before any
		action := "created"
		if creating {
			var e error
			result, e = scanGroup(tx.QueryRow(ctx, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,$2) RETURNING `+groupColumns, club, in.Name))
			if e != nil {
				return e
			}
		} else {
			old, e := scanGroup(tx.QueryRow(ctx, `SELECT `+groupColumns+` FROM pride.training_groups WHERE club_id::text=$1 AND id::text=$2 FOR UPDATE`, club, r.PathValue("groupID")))
			if e != nil {
				return e
			}
			if old.Version != in.Version {
				return ErrConflict
			}
			if old.Archived {
				return ErrArchived
			}
			before = groupState(old)
			action = "updated"
			result, e = scanGroup(tx.QueryRow(ctx, `UPDATE pride.training_groups SET name=$2,version=version+1,updated_at=now() WHERE id=$1 RETURNING `+groupColumns, old.ID, in.Name))
			if e != nil {
				return e
			}
		}
		return Audit(ctx, tx, actor, club, result.ID, "training_group", action, before, groupState(result))
	})
	if e != nil {
		Fail(w, e)
		return
	}
	code := 200
	if creating {
		code = 201
	}
	respond(w, code, result)
}
func (s *Service) archive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		Fail(w, ErrInput)
		return
	}
	var result Group
	e := s.transaction(r, func(tx pgx.Tx) error {
		ctx := r.Context()
		old, e := scanGroup(tx.QueryRow(ctx, `SELECT `+groupColumns+` FROM pride.training_groups WHERE club_id::text=$1 AND id::text=$2 FOR UPDATE`, r.PathValue("clubID"), r.PathValue("groupID")))
		if e != nil {
			return e
		}
		if old.Version != in.Version {
			return ErrConflict
		}
		result = old
		if old.Archived {
			return nil
		}
		result, e = scanGroup(tx.QueryRow(ctx, `UPDATE pride.training_groups SET archived_at=now(),version=version+1,updated_at=now() WHERE id=$1 RETURNING `+groupColumns, old.ID))
		if e != nil {
			return e
		}
		return Audit(ctx, tx, auth.CurrentUser(ctx).ID, old.ClubID, old.ID, "training_group", "archived", groupState(old), groupState(result))
	})
	if e != nil {
		Fail(w, e)
		return
	}
	respond(w, 200, result)
}
func (s *Service) intervals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	club := r.PathValue("clubID")
	group := r.PathValue("groupID")
	member := r.PathValue("membershipID")
	at := r.URL.Query().Get("date")
	var exists bool
	var e error
	if group != "" {
		e = s.pool.QueryRow(ctx, `SELECT true FROM pride.training_groups WHERE club_id::text=$1 AND id::text=$2`, club, group).Scan(&exists)
		if !ValidDate(at) {
			Fail(w, ErrInput)
			return
		}
	} else {
		e = s.pool.QueryRow(ctx, `SELECT true FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2`, club, member).Scan(&exists)
	}
	if e != nil {
		Fail(w, e)
		return
	}
	query := `SELECT ` + intervalColumns + intervalJoins + ` WHERE m.club_id::text=$1 AND m.club_membership_id::text=$2 ORDER BY m.valid_from,m.id`
	args := []any{club, member}
	if group != "" {
		query = `SELECT ` + intervalColumns + intervalJoins + ` WHERE m.club_id::text=$1 AND m.group_id::text=$2 AND m.valid_from<=$3::date AND (m.valid_until IS NULL OR $3::date<m.valid_until) ORDER BY lower(p.full_name),m.id`
		args = []any{club, group, at}
	}
	rows, e := s.pool.Query(ctx, query, args...)
	if e != nil {
		Fail(w, e)
		return
	}
	defer rows.Close()
	out := []Interval{}
	for rows.Next() {
		i, e := scanInterval(rows)
		if e != nil {
			Fail(w, e)
			return
		}
		out = append(out, i)
	}
	if e = rows.Err(); e != nil {
		Fail(w, e)
		return
	}
	respond(w, 200, out)
}
func (s *Service) add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GroupID string  `json:"groupId"`
		From    string  `json:"from"`
		Until   *string `json:"until"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !ValidDate(in.From) || (in.Until != nil && (!ValidDate(*in.Until) || *in.Until <= in.From)) {
		Fail(w, ErrInput)
		return
	}
	var result Interval
	e := s.transaction(r, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		member := r.PathValue("membershipID")
		archived, e := lockCard(ctx, tx, club, member)
		if e != nil {
			return e
		}
		if archived {
			return ErrArchived
		}
		if _, e = ReadGroup(ctx, tx, club, in.GroupID, true); e != nil {
			return e
		}
		result, e = insert(ctx, tx, auth.CurrentUser(ctx).ID, club, member, in.GroupID, in.From, in.Until)
		return e
	})
	if e != nil {
		Fail(w, e)
		return
	}
	respond(w, 201, result)
}
func (s *Service) end(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date    string `json:"date"`
		Version int    `json:"version"`
		GroupID string `json:"groupId,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	transfer := strings.HasSuffix(r.URL.Path, "/transfer")
	if !ValidDate(in.Date) || in.Version < 1 || (!transfer && in.GroupID != "") || (transfer && in.GroupID == "") {
		Fail(w, ErrInput)
		return
	}
	var result Interval
	e := s.transaction(r, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		member := r.PathValue("membershipID")
		archived, e := lockCard(ctx, tx, club, member)
		if e != nil {
			return e
		}
		old, e := readInterval(ctx, tx, club, r.PathValue("intervalID"))
		if e != nil {
			return e
		}
		if old.MembershipID != member {
			return pgx.ErrNoRows
		}
		if old.Version != in.Version {
			return ErrConflict
		}
		if in.Date <= old.From || (old.Until != nil && in.Date >= *old.Until) {
			return ErrInput
		}
		if transfer {
			if archived {
				return ErrArchived
			}
			if in.GroupID == old.GroupID {
				return ErrInput
			}
			if _, e = ReadGroup(ctx, tx, club, in.GroupID, true); e != nil {
				return e
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE pride.group_memberships SET valid_until=$2::date,version=version+1,updated_at=now() WHERE id=$1`, old.ID, in.Date); e != nil {
			return e
		}
		result, e = readInterval(ctx, tx, club, old.ID)
		if e != nil {
			return e
		}
		action := "excluded"
		if transfer {
			action = "transferred"
		}
		actor := auth.CurrentUser(ctx).ID
		if e = Audit(ctx, tx, actor, club, old.ID, "group_membership", action, intervalState(old), intervalState(result)); e != nil {
			return e
		}
		if transfer {
			result, e = insert(ctx, tx, actor, club, member, in.GroupID, in.Date, old.Until)
		}
		return e
	})
	if e != nil {
		Fail(w, e)
		return
	}
	respond(w, 200, result)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/clubs/{clubID}/groups", s.access(s.list))
	mux.Handle("POST /api/clubs/{clubID}/groups", s.access(s.save))
	mux.Handle("PUT /api/clubs/{clubID}/groups/{groupID}", s.access(s.save))
	mux.Handle("POST /api/clubs/{clubID}/groups/{groupID}/archive", s.access(s.archive))
	mux.Handle("GET /api/clubs/{clubID}/groups/{groupID}/roster", s.access(s.intervals))
	mux.Handle("GET /api/clubs/{clubID}/participants/{membershipID}/groups", s.access(s.intervals))
	mux.Handle("POST /api/clubs/{clubID}/participants/{membershipID}/groups", s.access(s.add))
	mux.Handle("POST /api/clubs/{clubID}/participants/{membershipID}/groups/{intervalID}/exclude", s.access(s.end))
	mux.Handle("POST /api/clubs/{clubID}/participants/{membershipID}/groups/{intervalID}/transfer", s.access(s.end))
	mux.Handle("/", fallback)
	return mux
}
