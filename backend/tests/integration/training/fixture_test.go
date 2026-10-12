package training_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/reports"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/corrections"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const origin = "https://training.example.test"
const password = "synthetic-training-test-only"

type client struct {
	h       http.Handler
	id      string
	cookies []*http.Cookie
	csrf    string
}

func (c client) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(ctx)
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "192.0.2.163:163"
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%s %s: personal response can be cached", method, path)
	}
	return w
}

func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status %d, want %d: %s", w.Code, want, w.Body.String())
	}
}

func parse[T any](t *testing.T, w *httptest.ResponseRecorder, want int) T {
	t.Helper()
	status(t, w, want)
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL training integration")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_training_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		admin.Close(ctx)
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "20000"
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

type branch struct {
	club, group string
}

func (b branch) base() string { return "/api/clubs/" + b.club }
func trainerPath(b branch, id string) string {
	return b.base() + "/trainer-sessions/" + id + "/journal"
}
func adminPath(b branch, id string) string { return b.base() + "/journals/" + id }

type fixture struct {
	pool      *pgxpool.Pool
	h         http.Handler
	generator *schedule.Service
	users     map[string]client
	a, b      branch
}

func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) id(t *testing.T, q string, args ...any) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(), q, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) login(t *testing.T, name string) client {
	t.Helper()
	c := client{h: f.h}
	w := c.call(t, "POST", "/api/auth/login", map[string]string{"login": name, "password": password})
	status(t, w, 200)
	c.cookies = w.Result().Cookies()
	me := parse[struct {
		User auth.User `json:"user"`
		CSRF string    `json:"csrfToken"`
	}](t, c.call(t, "GET", "/api/auth/me", nil), 200)
	c.csrf, c.id = me.CSRF, me.User.ID
	return c
}

func (f *fixture) staff(t *testing.T, name string, assignments ...string) {
	id := f.id(t, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name)
	for i := 0; i < len(assignments); i += 2 {
		f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, id, assignments[i], assignments[i+1])
	}
	f.users[name] = f.login(t, name)
}

func (f *fixture) session(t *testing.T, b branch, trainer, group, date string) string {
	return f.id(t, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4::date,'Europe/Minsk',($4::date+TIME '18:00') AT TIME ZONE 'Europe/Minsk',($4::date+TIME '19:00') AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, b.club, group, trainer, date)
}

func (f *fixture) member(t *testing.T, b branch, name, from string, until *string) string {
	p := f.id(t, `INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'1990-01-01') RETURNING id::text`, name)
	m := f.id(t, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'1990-01-01','phone','synthetic-private-contact') RETURNING id::text`, p, b.club)
	f.exec(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from,valid_until) VALUES($1,$2,$3,$4::date,$5::date)`, b.club, b.group, m, from, until)
	return m
}
func (f *fixture) group(t *testing.T, club string) string {
	return f.id(t, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Same synthetic group') RETURNING id::text`, club)
}
func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{pool: database(t), users: map[string]client{}}
	if err := auth.CreateFirstOwner(context.Background(), f.pool, "owner", "synthetic-owner", password); err != nil {
		t.Fatal(err)
	}
	access, err := auth.New(f.pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	f.generator = schedule.New(f.pool, access)
	h := corrections.New(f.pool, access).Handler(http.NotFoundHandler())
	h = reports.New(f.pool, access).Handler(h)
	f.h = access.Handler(attendance.New(f.pool, access).Handler(sessions.New(f.pool, access).Handler(f.generator.Handler(groups.New(f.pool, access).Handler(clubs.New(f.pool, access).Handler(h))))))
	f.users["owner"] = f.login(t, "owner")
	for i, b := range []*branch{&f.a, &f.b} {
		b.club = parse[clubs.Club](t, f.users["owner"].call(t, "POST", "/api/clubs", map[string]string{"name": fmt.Sprintf("Branch %d", i), "timezone": "Europe/Minsk"}), 201).ID
		b.group = f.group(t, b.club)
	}
	f.staff(t, "admin", f.a.club, "administrator")
	f.staff(t, "trainer", f.a.club, "trainer")
	f.staff(t, "replacement", f.a.club, "trainer")
	f.staff(t, "foreign", f.b.club, "trainer")
	return f
}
func ptr(s string) *string         { return &s }
func version(n int) map[string]any { return map[string]any{"version": n} }
func mark(j attendance.Journal, i int, state string, reason *string) map[string]any {
	return map[string]any{"version": j.Records[i].Version, "sessionVersion": j.Session.Version, "state": state, "absenceReason": reason}
}
func (f *fixture) open(t *testing.T, b branch, id string) attendance.Journal {
	return parse[attendance.Journal](t, f.users["trainer"].call(t, "POST", trainerPath(b, id)+"/open", version(1)), 200)
}
func (f *fixture) journal(t *testing.T, b branch, id string) attendance.Journal {
	return parse[attendance.Journal](t, f.users["owner"].call(t, "GET", adminPath(b, id), nil), 200)
}
func (f *fixture) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Full domain fingerprints prove rollback, including versions and audit payloads.
func (f *fixture) fingerprint(t *testing.T) string {
	t.Helper()
	var result strings.Builder
	for _, table := range []string{"schedule_rules", "training_sessions", "attendance_records", "audit_events"} {
		result.WriteString(f.id(t, `SELECT coalesce(jsonb_agg(to_jsonb(v) ORDER BY id)::text,'[]') FROM pride.`+table+` v`))
	}
	return result.String()
}
func (f *fixture) snapshot(t *testing.T, id string) string {
	return f.id(t, `SELECT coalesce(jsonb_agg(jsonb_build_array(id,club_id,training_session_id,group_id,club_membership_id,source_group_membership_id,created_at) ORDER BY id)::text,'[]') FROM pride.attendance_records WHERE training_session_id=$1`, id)
}
