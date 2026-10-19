// Package notifications serves a personal inbox with live domain authorization.
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }

type Notification struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Href      string    `json:"href"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}
type Inbox struct {
	Items       []Notification `json:"items"`
	Total       int            `json:"total"`
	HasMore     bool           `json:"hasMore"`
	UnreadCount int            `json:"unreadCount"`
}

func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/notifications", s.auth.Require(http.HandlerFunc(s.list)))
	mux.Handle("POST /api/notifications/read-all", s.auth.Require(http.HandlerFunc(s.read)))
	mux.Handle("POST /api/notifications/{id}/read", s.auth.Require(http.HandlerFunc(s.read)))
	mux.Handle("/", next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/notifications" || strings.HasPrefix(r.URL.Path, "/api/notifications/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		mux.ServeHTTP(w, r)
	})
}

var errInput = errors.New("invalid inbox input")

func fail(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте фильтр и страницу уведомлений.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Уведомление недоступно.")
	case errors.As(err, &p) && (p.Code == "40001" || p.Code == "40P01"):
		platform.WriteError(w, 409, "version_conflict", "Список изменился. Обновите уведомления.")
	default:
		platform.WriteError(w, 503, "not_ready", "Уведомления временно недоступны.")
	}
}
func respond(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

// The source family is retained at delivery. Moving a child to another family
// must neither expose old messages to the new family nor preserve the old access.
// User/session locks serialize account/role revocation; the entire inbox query
// below uses ONE SQL snapshot for links, sessions, items, totals and unreadCount.
const manager = `($2 OR EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=$1 AND r.club_id=n.club_id AND r.role='administrator'))`
const scope = `n.user_id=$1 AND (` + manager + `
 OR (n.family_id IS NOT NULL AND EXISTS(SELECT 1 FROM pride.family_access a
   JOIN pride.clubs c ON c.id=n.club_id AND c.archived_at IS NULL
   WHERE a.user_id=$1 AND a.family_id=n.family_id AND (n.membership_id IS NULL OR EXISTS(
     SELECT 1 FROM pride.family_children fc WHERE fc.family_id=n.family_id AND fc.membership_id=n.membership_id AND fc.club_id=n.club_id))))
 OR (n.session_id IS NOT NULL AND EXISTS(SELECT 1 FROM pride.training_sessions t
   JOIN pride.staff_club_roles r ON r.club_id=t.club_id AND r.staff_user_id=$1 AND r.role='trainer'
   JOIN pride.clubs c ON c.id=t.club_id AND c.archived_at IS NULL
   WHERE t.id=n.session_id AND t.trainer_id=$1 AND t.superseded_at IS NULL)))`

func (s *Service) transaction(r *http.Request, fn func(pgx.Tx, string, bool) error) error {
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	owner, err := auth.LockSession(r.Context(), tx)
	if err != nil {
		return err
	}
	if err = fn(tx, auth.CurrentUser(r.Context()).ID, owner); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	page, limit, status := 0, 25, "all"
	for key, values := range r.URL.Query() {
		if len(values) != 1 {
			fail(w, errInput)
			return
		}
		var err error
		switch key {
		case "page":
			page, err = strconv.Atoi(values[0])
		case "limit":
			limit, err = strconv.Atoi(values[0])
		case "status":
			status = values[0]
		default:
			err = errInput
		}
		if err != nil {
			fail(w, errInput)
			return
		}
	}
	if page < 0 || page > 1000000 || limit < 1 || limit > 100 || (status != "all" && status != "unread") {
		fail(w, errInput)
		return
	}
	var result Inbox
	err := s.transaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		// href is constructed solely from server UUIDs and fixed internal paths,
		// never from audit text or a stored/client-supplied URL. Parent links use
		// portal routes; managers and trainers use the current club context.
		var raw []byte
		err := tx.QueryRow(r.Context(), `WITH visible AS MATERIALIZED (
 SELECT n.*,`+manager+` AS manager FROM pride.notifications n WHERE `+scope+`
), filtered AS MATERIALIZED (SELECT * FROM visible WHERE $3='all' OR read_at IS NULL),
page AS (SELECT n.id,n.kind,n.title,n.body,n.read_at IS NOT NULL AS read,n.created_at,
 CASE WHEN n.kind='training_session' THEN '/clubs/'||n.club_id||'/sessions?session='||n.session_id||
   coalesce((SELECT '&from='||to_char(t.starts_at AT TIME ZONE t.timezone,'YYYY-MM-DD')||'&through='||to_char(t.starts_at AT TIME ZONE t.timezone,'YYYY-MM-DD') FROM pride.training_sessions t WHERE t.id=n.session_id),'')
 ELSE (CASE WHEN n.manager THEN '/clubs/'||n.club_id ELSE '' END)||
   CASE WHEN n.kind='payment' THEN '/payments' ELSE '/families'||
     CASE WHEN n.family_id IS NULL THEN '' ELSE '?detail='||n.family_id END END END AS href
 FROM filtered n ORDER BY n.created_at DESC,n.id DESC LIMIT $4 OFFSET $5)
 SELECT jsonb_build_object('items',coalesce((SELECT jsonb_agg(jsonb_build_object(
   'id',p.id,'kind',p.kind,'title',p.title,'body',p.body,'href',coalesce(p.href,'/notifications'),
   'read',p.read,'createdAt',p.created_at) ORDER BY p.created_at DESC,p.id DESC) FROM page p),'[]'::jsonb),
 'total',(SELECT count(*) FROM filtered),'unreadCount',(SELECT count(*) FROM visible WHERE read_at IS NULL),
 'hasMore',$5+(SELECT count(*) FROM page)<(SELECT count(*) FROM filtered))`, actor, owner, status, limit, page*limit).Scan(&raw)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &result)
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, result)
}

func (s *Service) read(w http.ResponseWriter, r *http.Request) {
	ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var body map[string]json.RawMessage
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	if err != nil || ct != "application/json" || d.Decode(&body) != nil || body == nil || len(body) != 0 || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return
	}
	id := r.PathValue("id")
	var uuid pgtype.UUID
	if id != "" && (uuid.Scan(id) != nil || !uuid.Valid) {
		fail(w, pgx.ErrNoRows)
		return
	}
	err = s.transaction(r, func(tx pgx.Tx, actor string, owner bool) error {
		// Include already-read rows in the update so a repeated read returns 200;
		// coalesce preserves the first read time. Hidden/foreign IDs always 404.
		result, err := tx.Exec(r.Context(), `UPDATE pride.notifications n SET read_at=coalesce(n.read_at,now())
 WHERE `+scope+` AND ($3::uuid IS NULL OR n.id=$3::uuid) AND ($3::uuid IS NOT NULL OR n.read_at IS NULL)`, actor, owner, nilIfEmpty(id))
		if err == nil && id != "" && result.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return err
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, map[string]string{"status": "ok"})
}
func nilIfEmpty(id string) any {
	if id == "" {
		return nil
	}
	return id
}
