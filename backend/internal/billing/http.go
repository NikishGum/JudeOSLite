// Package billing implements club-local manual BYN finances. It never contacts a gateway.
package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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

var errInput = errors.New("invalid billing input")
var errConflict = errors.New("billing conflict")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	routes := map[string]http.HandlerFunc{
		"GET /api/clubs/{clubID}/billing/plans":                 s.plans,
		"POST /api/clubs/{clubID}/billing/plans":                s.savePlan,
		"PUT /api/clubs/{clubID}/billing/plans/{planID}":        s.savePlan,
		"GET /api/clubs/{clubID}/payments":                      s.payments,
		"POST /api/clubs/{clubID}/payments":                     s.createPayment,
		"POST /api/clubs/{clubID}/payments/{paymentID}/confirm": s.changePayment,
		"POST /api/clubs/{clubID}/payments/{paymentID}/void":    s.changePayment,
		"GET /api/clubs/{clubID}/subscriptions":                 s.subscriptions,
		"GET /api/clubs/{clubID}/billing/summary":               s.summary,
		"GET /api/portal/payments":                              s.payments,
		"GET /api/portal/subscriptions":                         s.subscriptions,
	}
	for route, handler := range routes {
		mux.Handle(route, s.auth.Require(handler))
	}
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/billing/") || strings.HasSuffix(r.URL.Path, "/payments") || strings.Contains(r.URL.Path, "/payments/") || strings.HasSuffix(r.URL.Path, "/subscriptions") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
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
		platform.WriteError(w, 400, "invalid_input", "Проверьте поля, сумму и даты.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Запись или филиал недоступны.")
	case errors.Is(err, errConflict), errors.Is(err, clubs.ErrArchived), errors.As(err, &p) && (p.Code == "23505" || p.Code == "23P01" || p.Code == "40001" || p.Code == "40P01"):
		platform.WriteError(w, 409, "version_conflict", "Запись изменена, ключ запроса занят или интервалы пересекаются. Обновите данные.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any, required ...string) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail(w, errInput)
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16*1024))
	if err != nil {
		fail(w, errInput)
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		fail(w, errInput)
		return false
	}
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			fail(w, errInput)
			return false
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return false
	}
	return true
}
func validText(v string, max int) bool {
	if !utf8.ValidString(v) || utf8.RuneCountInString(v) > max {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func uuid(v string) bool { var id pgtype.UUID; return id.Scan(v) == nil && id.Valid }
func date(v string) bool {
	t, e := time.Parse("2006-01-02", v)
	return e == nil && t.Year() >= 1 && t.Format("2006-01-02") == v
}
func method(v string) bool { return v == "cash" || v == "card" || v == "transfer" || v == "erip" }
func (s *Service) transaction(r *http.Request, portal, write bool, fn func(pgx.Tx) error) error {
	if !portal {
		for _, key := range []string{"clubID", "planID", "paymentID"} {
			value := r.PathValue(key)
			if (key == "clubID" || value != "") && !uuid(value) {
				return pgx.ErrNoRows
			}
		}
	}
	options := pgx.TxOptions{}
	if !write {
		// Totals, rows, month buckets and attendance usage share one snapshot.
		// Live authorization/link locks still linearize against revoke/unlink.
		options.IsoLevel = pgx.RepeatableRead
	}
	tx, err := s.pool.BeginTx(r.Context(), options)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if portal {
		_, err = auth.LockSession(r.Context(), tx)
	} else {
		_, err = auth.LockClubAdministrator(r.Context(), tx, r.PathValue("clubID"))
		if err == nil {
			var ok bool
			err = tx.QueryRow(r.Context(), `SELECT true FROM pride.clubs WHERE id=$1::uuid FOR SHARE`, r.PathValue("clubID")).Scan(&ok)
		}
	}
	if err != nil {
		return err
	}
	if write {
		if err = clubs.RequireActive(r.Context(), tx, r.PathValue("clubID")); err != nil {
			return err
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

type filters struct {
	Page, Limit                    int
	Q, Status, From, Until, Member string
}

func query(r *http.Request) (filters, error) {
	q := r.URL.Query()
	f := filters{Limit: 25, Q: q.Get("q"), Status: q.Get("status"), From: q.Get("from"), Until: q.Get("until"), Member: q.Get("membershipId")}
	if f.Status == "all" {
		f.Status = ""
	}
	var err error
	if q.Has("page") {
		f.Page, err = strconv.Atoi(q.Get("page"))
		if err != nil {
			return f, errInput
		}
	}
	if q.Has("limit") {
		f.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			return f, errInput
		}
	}
	if f.Page < 0 || f.Page > 1000000 || f.Limit < 1 || f.Limit > 100 || !validText(f.Q, 200) || (f.Member != "" && !uuid(f.Member)) || (f.From != "" && !date(f.From)) || (f.Until != "" && !date(f.Until)) || (f.From != "" && f.Until != "" && f.From > f.Until) {
		return f, errInput
	}
	return f, nil
}
func listResponse(w http.ResponseWriter, items any, total, page, limit, n int) {
	respond(w, 200, map[string]any{"items": items, "total": total, "hasMore": page*limit+n < total})
}

// Only identifiers, versions and financial state enter audit; no family contact data.
// RD-12 can consume these committed events in its own migration.
func audit(ctx context.Context, tx pgx.Tx, club, id, entity, action string, before, after any) error {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	_, err := tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7)`, club, auth.CurrentUser(ctx).ID, entity, id, action, b, a)
	return err
}
