package access_test

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

	"github.com/NikishGum/JudeOSLite/backend/internal/audit"
	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/participants"
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

const origin = "https://access.example.test"
const password = "synthetic-access-test-only"
const missingID = "00000000-0000-0000-0000-000000000001"
const period = "?from=2000-02-01&until=2000-03-01"

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
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "192.0.2.162:162"
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

func excludes(t *testing.T, w *httptest.ResponseRecorder, values ...string) {
	t.Helper()
	for _, value := range values {
		if value != "" && strings.Contains(w.Body.String(), value) {
			t.Fatalf("response leaked %q: %s", value, w.Body.String())
		}
	}
}

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL access matrix")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_access_%d", time.Now().UnixNano())
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
	club, group, member, interval, rule string
	contact, name                       string
	open, closed                        attendance.Journal
}

func (b branch) base() string { return "/api/clubs/" + b.club }
func trainerPath(b branch, id string) string {
	return b.base() + "/trainer-sessions/" + id + "/journal"
}
func adminPath(b branch, id string) string { return b.base() + "/journals/" + id }

type fixture struct {
	pool                                                                    *pgxpool.Pool
	h                                                                       http.Handler
	users                                                                   map[string]client
	a, b                                                                    branch
	privateGroup, privateRule, privateSession, multiSessionA, multiSessionB string
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

func (f *fixture) rule(t *testing.T, b branch, group, trainer string) string {
	return f.id(t, `INSERT INTO pride.schedule_rules(club_id,group_id,trainer_id,mode,weekdays,starts_time,duration_minutes,timezone,valid_from) VALUES($1,$2,$3,'weekly',ARRAY[1],TIME '18:00',60,'Europe/Minsk','2000-01-01') RETURNING id::text`, b.club, group, trainer)
}

func (f *fixture) seed(t *testing.T, b *branch, trainer string) {
	b.group = f.id(t, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,$2) RETURNING id::text`, b.club, "group-"+b.name)
	p := f.id(t, `INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'1990-01-01') RETURNING id::text`, "participant-"+b.name)
	b.member = f.id(t, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value,contact_name) VALUES($1,$2,'2000-01-01','email',$3,$4) RETURNING id::text`, p, b.club, b.contact, "parent-"+b.name)
	b.interval = f.id(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2000-01-01') RETURNING id::text`, b.club, b.group, b.member)
	b.rule = f.rule(t, *b, b.group, f.users[trainer].id)
	c := f.users[trainer]
	for i, target := range []*attendance.Journal{&b.closed, &b.open} {
		id := f.session(t, *b, c.id, b.group, fmt.Sprintf("2000-02-%02d", i+1))
		*target = parse[attendance.Journal](t, c.call(t, "POST", trainerPath(*b, id)+"/open", map[string]any{"version": 1}), 200)
		if i == 0 {
			*target = parse[attendance.Journal](t, c.call(t, "POST", trainerPath(*b, id)+"/close", map[string]any{"version": target.Session.Version}), 200)
		}
	}
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
	// Same domain handler composition as cmd/server; every request uses live auth.
	h := corrections.New(f.pool, access).Handler(http.NotFoundHandler())
	h = audit.New(f.pool, access).Handler(h)
	h = reports.New(f.pool, access).Handler(h)
	f.h = access.Handler(attendance.New(f.pool, access).Handler(sessions.New(f.pool, access).Handler(schedule.New(f.pool, access).Handler(groups.New(f.pool, access).Handler(participants.New(f.pool, access).Handler(clubs.New(f.pool, access).Handler(h)))))))
	f.users["owner"] = f.login(t, "owner")
	for i, b := range []*branch{&f.a, &f.b} {
		b.name = fmt.Sprintf("private-branch-%d", i)
		b.contact = fmt.Sprintf("synthetic-private-%d@example.test", i)
		b.club = parse[clubs.Club](t, f.users["owner"].call(t, "POST", "/api/clubs", map[string]string{"name": b.name, "timezone": "Europe/Minsk"}), 201).ID
	}
	f.staff(t, "admin-a", f.a.club, "administrator")
	f.staff(t, "admin-b", f.b.club, "administrator")
	f.staff(t, "admin-both", f.a.club, "administrator", f.b.club, "administrator")
	f.staff(t, "trainer-a", f.a.club, "trainer")
	f.staff(t, "trainer-b", f.b.club, "trainer")
	f.staff(t, "trainer-both", f.a.club, "trainer", f.b.club, "trainer")
	f.staff(t, "mixed", f.a.club, "administrator", f.b.club, "trainer")
	f.staff(t, "replacement", f.a.club, "trainer")
	f.staff(t, "unassigned")
	f.users["anonymous"] = client{h: f.h}
	f.seed(t, &f.a, "trainer-a")
	f.seed(t, &f.b, "trainer-b")
	f.privateGroup = f.id(t, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,'unassigned-private-group') RETURNING id::text`, f.a.club)
	f.privateRule = f.rule(t, f.a, f.privateGroup, f.users["replacement"].id)
	f.privateSession = f.session(t, f.a, f.users["replacement"].id, f.privateGroup, "2000-02-04")
	f.multiSessionA = f.session(t, f.a, f.users["trainer-both"].id, f.a.group, "2000-02-03")
	f.multiSessionB = f.session(t, f.b, f.users["trainer-both"].id, f.b.group, "2000-02-03")
	return f
}

// Fingerprint every domain row, including versions, secrets, roles and audit.
// Denied commands must not partially mutate data or append an audit entry.
func (f *fixture) fingerprint(t *testing.T) string {
	t.Helper()
	var result strings.Builder
	for _, table := range []string{"clubs", "staff_users", "staff_club_roles", "staff_sessions", "participants", "club_memberships", "training_groups", "group_memberships", "schedule_rules", "training_sessions", "attendance_records", "audit_events"} {
		result.WriteString(f.id(t, `SELECT coalesce(jsonb_agg(to_jsonb(v) ORDER BY id)::text,'[]') FROM pride.`+table+` v`))
	}
	return result.String()
}
