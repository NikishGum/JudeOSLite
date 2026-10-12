// Package audit exposes only the public envelope of existing audit records.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pageSize = 50

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var actionPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,79}$`)
var errInput = errors.New("invalid audit filter")
var errForbidden = errors.New("audit access denied")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/audit/filters", s.auth.Require(http.HandlerFunc(s.filters)))
	mux.Handle("GET /api/audit/events", s.auth.Require(http.HandlerFunc(s.events)))
	mux.Handle("/", next)
	return mux
}
func respond(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте фильтры истории и обновите список.")
	case errors.Is(err, errForbidden):
		platform.WriteError(w, 403, "forbidden", "История доступна владельцу и администраторам своих филиалов.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "История филиала недоступна.")
	default:
		platform.WriteError(w, 503, "not_ready", "История временно недоступна. Попробуйте снова.")
	}
}

// Actor/role/session locks serialize reads with disable, session revocation and
// assignment changes (the assignment trigger locks staff_users). No cached role
// or client-provided user identity is used for deciding the scope.
func (s *Service) begin(r *http.Request) (pgx.Tx, string, bool, error) {
	ctx := r.Context()
	actor := auth.CurrentUser(ctx).ID
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", false, err
	}
	var club string
	var owner bool
	err = tx.QueryRow(ctx, `SELECT is_owner,coalesce((SELECT club_id::text FROM pride.staff_club_roles WHERE staff_user_id=u.id AND role='administrator' ORDER BY club_id LIMIT 1),'') FROM pride.staff_users u WHERE id=$1 AND disabled_at IS NULL`, actor).Scan(&owner, &club)
	if err == nil && !owner && club == "" {
		err = errForbidden
	}
	if err == nil {
		owner, err = auth.LockClubAdministrator(ctx, tx, club)
	}
	if err != nil {
		_ = tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			err = errForbidden
		}
		return nil, "", false, err
	}
	return tx, actor, owner, nil
}

const scope = `($2::boolean OR (e.club_id IS NOT NULL AND EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=$1 AND r.club_id=e.club_id AND r.role='administrator')))`

type Club struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}
type Filters struct {
	Owner   bool     `json:"isOwner"`
	Clubs   []Club   `json:"clubs"`
	Actions []string `json:"actions"`
}

func (s *Service) filters(w http.ResponseWriter, r *http.Request) {
	tx, actor, owner, err := s.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	result := Filters{Owner: owner, Clubs: []Club{}, Actions: []string{}}
	rows, err := tx.Query(r.Context(), `SELECT c.id::text,c.name,c.archived_at IS NOT NULL FROM pride.clubs c WHERE $2 OR EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=$1 AND r.club_id=c.id AND r.role='administrator') ORDER BY c.name,c.id`, actor, owner)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var c Club
		if err = rows.Scan(&c.ID, &c.Name, &c.Archived); err != nil {
			break
		}
		result.Clubs = append(result.Clubs, c)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, err)
		return
	}
	rows, err = tx.Query(r.Context(), `SELECT DISTINCT e.action FROM pride.audit_events e WHERE `+scope+` ORDER BY e.action`, actor, owner)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var a string
		if err = rows.Scan(&a); err != nil {
			break
		}
		if actionPattern.MatchString(a) {
			result.Actions = append(result.Actions, a)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, result)
}

type cursor struct {
	At     time.Time `json:"at"`
	ID     string    `json:"id"`
	Filter string    `json:"filter"`
}
type query struct {
	club, action, fingerprint string
	from, until               *time.Time
	cursor                    *cursor
}

func parseQuery(values url.Values) (query, error) {
	q := query{club: values.Get("clubId"), action: values.Get("action")}
	for k, v := range values {
		if len(v) != 1 || (k != "clubId" && k != "action" && k != "from" && k != "until" && k != "cursor") {
			return q, errInput
		}
	}
	if q.club != "" && q.club != "network" && !uuid.MatchString(q.club) {
		return q, errInput
	}
	q.club = strings.ToLower(q.club)
	if q.action != "" && !actionPattern.MatchString(q.action) {
		return q, errInput
	}
	loc, _ := time.LoadLocation("Europe/Minsk")
	for _, field := range []string{"from", "until"} {
		raw := values.Get(field)
		if raw == "" {
			continue
		}
		d, e := time.ParseInLocation("2006-01-02", raw, loc)
		if e != nil || len(raw) != 10 || d.Year() < 1 {
			return q, errInput
		}
		if field == "from" {
			q.from = &d
		} else {
			d = d.AddDate(0, 0, 1)
			q.until = &d
		}
	}
	if q.from != nil && q.until != nil && !q.from.Before(*q.until) {
		return q, errInput
	}
	sum := sha256.Sum256([]byte(q.club + "|" + q.action + "|" + values.Get("from") + "|" + values.Get("until")))
	q.fingerprint = hex.EncodeToString(sum[:])
	if raw := values.Get("cursor"); raw != "" {
		if len(raw) > 600 {
			return q, errInput
		}
		b, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil {
			return q, errInput
		}
		var c cursor
		if json.Unmarshal(b, &c) != nil || c.At.IsZero() || !uuid.MatchString(c.ID) || c.Filter != q.fingerprint {
			return q, errInput
		}
		q.cursor = &c
	}
	return q, nil
}

type Event struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"createdAt"`
	ActorID       string    `json:"actorId"`
	ActorName     string    `json:"actorName"`
	ActorDisabled bool      `json:"actorDisabled"`
	ClubID        *string   `json:"clubId"`
	ClubName      *string   `json:"clubName"`
	ClubArchived  bool      `json:"clubArchived"`
	EntityType    string    `json:"entityType"`
	EntityID      string    `json:"entityId"`
	Action        string    `json:"action"`
}
type Page struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"nextCursor"`
}

func (s *Service) events(w http.ResponseWriter, r *http.Request) {
	q, err := parseQuery(r.URL.Query())
	if err != nil {
		fail(w, err)
		return
	}
	tx, actor, owner, err := s.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	if q.club == "network" && !owner {
		fail(w, pgx.ErrNoRows)
		return
	}
	if q.club != "" && q.club != "network" {
		var allowed bool
		err = tx.QueryRow(r.Context(), `SELECT true FROM pride.clubs c WHERE c.id::text=$3 AND ($2 OR EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=$1 AND r.club_id=c.id AND r.role='administrator'))`, actor, owner, q.club).Scan(&allowed)
		if err != nil {
			fail(w, err)
			return
		}
	}
	var at *time.Time
	var id string
	if q.cursor != nil {
		at = &q.cursor.At
		id = q.cursor.ID
	}
	rows, err := tx.Query(r.Context(), `SELECT e.id::text,e.created_at,e.actor_id::text,u.display_name,u.disabled_at IS NOT NULL,e.club_id::text,c.name,coalesce(c.archived_at IS NOT NULL,false),e.entity_type,e.entity_id::text,e.action FROM pride.audit_events e JOIN pride.staff_users u ON u.id=e.actor_id LEFT JOIN pride.clubs c ON c.id=e.club_id WHERE `+scope+` AND ($3='' OR ($3='network' AND e.club_id IS NULL) OR e.club_id::text=$3) AND ($4='' OR e.action=$4) AND ($5::timestamptz IS NULL OR e.created_at>=$5) AND ($6::timestamptz IS NULL OR e.created_at<$6) AND ($7::timestamptz IS NULL OR (e.created_at,e.id)<($7,$8::uuid)) ORDER BY e.created_at DESC,e.id DESC LIMIT 51`, actor, owner, q.club, q.action, q.from, q.until, at, nullableID(id))
	if err != nil {
		fail(w, err)
		return
	}
	result := Page{Items: []Event{}}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.ID, &e.CreatedAt, &e.ActorID, &e.ActorName, &e.ActorDisabled, &e.ClubID, &e.ClubName, &e.ClubArchived, &e.EntityType, &e.EntityID, &e.Action); err != nil {
			break
		}
		result.Items = append(result.Items, e)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	if len(result.Items) > pageSize {
		result.Items = result.Items[:pageSize]
		last := result.Items[pageSize-1]
		b, _ := json.Marshal(cursor{last.CreatedAt, last.ID, q.fingerprint})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	respond(w, result)
}
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
