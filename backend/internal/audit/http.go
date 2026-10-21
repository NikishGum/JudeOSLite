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
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"
	"unicode/utf8"

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
	Owner        bool              `json:"isOwner"`
	Clubs        []Club            `json:"clubs"`
	Actions      []string          `json:"actions"`
	Actors       []Actor           `json:"actors"`
	ActionLabels map[string]string `json:"actionLabels"`
	EntityLabels map[string]string `json:"entityLabels"`
}

func (s *Service) filters(w http.ResponseWriter, r *http.Request) {
	tx, actor, owner, err := s.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	result := Filters{Owner: owner, Clubs: []Club{}, Actions: []string{}, Actors: []Actor{}, ActionLabels: labels(actionCatalog), EntityLabels: labels(entityCatalog)}
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
	if err != nil {
		fail(w, err)
		return
	}
	rows, err = tx.Query(r.Context(), `SELECT DISTINCT u.id::text,u.display_name,u.disabled_at IS NOT NULL FROM pride.audit_events e JOIN pride.staff_users u ON u.id=e.actor_id WHERE `+scope+` ORDER BY u.display_name,u.id::text`, actor, owner)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var a Actor
		if err = rows.Scan(&a.ID, &a.Name, &a.Disabled); err != nil {
			break
		}
		result.Actors = append(result.Actors, a)
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
	club, action, actor, search, fingerprint string
	terms                                    []string
	from, until                              *time.Time
	cursor                                   *cursor
}

func parseQuery(values url.Values) (query, error) {
	q := query{club: values.Get("clubId"), action: values.Get("action"), actor: strings.ToLower(values.Get("actorId"))}
	for k, v := range values {
		if len(v) != 1 || (k != "clubId" && k != "action" && k != "from" && k != "until" && k != "cursor" && k != "q" && k != "actorId") {
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
	if q.actor != "" && !uuid.MatchString(q.actor) {
		return q, errInput
	}
	rawSearch := values.Get("q")
	if !utf8.ValidString(rawSearch) || utf8.RuneCountInString(rawSearch) > 160 {
		return q, errInput
	}
	for _, ch := range rawSearch {
		if unicode.IsControl(ch) && !unicode.IsSpace(ch) {
			return q, errInput
		}
	}
	for _, term := range strings.Fields(strings.ReplaceAll(strings.ToLower(rawSearch), "ё", "е")) {
		term = strings.TrimPrefix(term, "#")
		if term != "" {
			q.terms = append(q.terms, term)
		}
	}
	sort.Strings(q.terms)
	q.search = strings.Join(q.terms, " ")
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
	fingerprint := q.club + "|" + q.action + "|" + values.Get("from") + "|" + values.Get("until")
	// Keep legacy cursors valid for unchanged legacy queries.
	if q.search != "" || q.actor != "" {
		encoded, _ := json.Marshal([]string{q.club, q.action, values.Get("from"), values.Get("until"), q.actor, q.search})
		fingerprint = string(encoded)
	}
	sum := sha256.Sum256([]byte(fingerprint))
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
	ActionLabel   string    `json:"actionLabel"`
	EntityLabel   string    `json:"entityLabel"`
}
type Page struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"nextCursor"`
	Total      int64   `json:"total"`
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
	// One statement gives items and total the same MVCC snapshot. The count is
	// before the keyset condition, including when that page has become empty.
	actionJSON, _ := json.Marshal(actionCatalog)
	entityJSON, _ := json.Marshal(entityCatalog)
	rows, err := tx.Query(r.Context(), `WITH matched AS MATERIALIZED (
 SELECT e.id, e.created_at AS "createdAt",e.actor_id AS "actorId",u.display_name AS "actorName",u.disabled_at IS NOT NULL AS "actorDisabled",
 e.club_id AS "clubId",c.name AS "clubName",coalesce(c.archived_at IS NOT NULL,false) AS "clubArchived",
 e.entity_type AS "entityType",e.entity_id AS "entityId",e.action,
 coalesce($9::jsonb->e.action->>'label',e.action) AS "actionLabel",
 coalesce($10::jsonb->e.entity_type->>'label',e.entity_type) AS "entityLabel"
 FROM pride.audit_events e JOIN pride.staff_users u ON u.id=e.actor_id LEFT JOIN pride.clubs c ON c.id=e.club_id
 WHERE `+scope+` AND ($3='' OR ($3='network' AND e.club_id IS NULL) OR e.club_id::text=$3)
 AND ($4='' OR e.action=$4) AND ($5::timestamptz IS NULL OR e.created_at>=$5) AND ($6::timestamptz IS NULL OR e.created_at<$6)
 AND ($12='' OR e.actor_id::text=$12)
 AND NOT EXISTS (SELECT 1 FROM unnest($11::text[]) term WHERE strpos(
 translate(lower(concat_ws(' ',u.display_name,coalesce(c.name,'Сеть без филиала'),e.action,
 $9::jsonb->e.action->>'label',$9::jsonb->e.action->>'aliases',e.entity_type,
 $10::jsonb->e.entity_type->>'label',$10::jsonb->e.entity_type->>'aliases',e.entity_id::text,e.id::text)),'ё','е'),term)=0)
 ) SELECT totals.total,row_to_json(p) FROM (SELECT count(*) total FROM matched) totals LEFT JOIN LATERAL (
 SELECT * FROM matched WHERE ($7::timestamptz IS NULL OR ("createdAt",id)<($7,$8::uuid))
 ORDER BY "createdAt" DESC,id DESC LIMIT 51) p ON true ORDER BY p."createdAt" DESC,p.id DESC`, actor, owner, q.club, q.action, q.from, q.until, at, nullableID(id), string(actionJSON), string(entityJSON), q.terms, q.actor)
	if err != nil {
		fail(w, err)
		return
	}
	result := Page{Items: []Event{}}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&result.Total, &raw); err != nil {
			break
		}
		if len(raw) == 0 {
			continue
		}
		var e Event
		if err = json.Unmarshal(raw, &e); err != nil {
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
