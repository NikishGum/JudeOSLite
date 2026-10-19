package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/NikishGum/JudeOSLite/backend/internal/participants"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/corrections"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-parent-password"
const origin = "https://families.example.test"

type client struct {
	cookies []*http.Cookie
	csrf    string
}
type fixture struct {
	t                                 *testing.T
	pool                              *pgxpool.Pool
	handler                           http.Handler
	owner, admin, trainer             client
	a, b, ownerID, adminID, trainerID string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("rd10_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, e := conn.Exec(ctx, "DROP DATABASE "+quoted+" WITH(FORCE)")
		if e != nil {
			t.Error(e)
		}
		conn.Close(ctx)
	})
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, pool: pool}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	f.ownerID = f.id(`INSERT INTO pride.staff_users(login,display_name,password_hash,is_owner) VALUES('owner','Owner',$1,true) RETURNING id::text`, hash)
	f.adminID = f.id(`INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('admin','Admin',$1) RETURNING id::text`, hash)
	f.trainerID = f.id(`INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('trainer','Trainer',$1) RETURNING id::text`, hash)
	f.a = f.id(`INSERT INTO pride.clubs(name) VALUES('A') RETURNING id::text`)
	f.b = f.id(`INSERT INTO pride.clubs(name) VALUES('B') RETURNING id::text`)
	f.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator'),($3,$2,'trainer')`, f.adminID, f.a, f.trainerID)
	access, err := auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
	})
	f.handler = access.Handler(New(pool, access).Handler(families.New(pool, access).Handler(corrections.New(pool, access).Handler(attendance.New(pool, access).Handler(participants.New(pool, access).Handler(clubs.New(pool, access).Handler(fallback)))))))
	f.owner = f.login("owner", password)
	f.admin = f.login("admin", password)
	f.trainer = f.login("trainer", password)
	return f
}
func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) id(sql string, args ...any) string {
	f.t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}
func (f *fixture) call(c client, method, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.RemoteAddr = "192.0.2.3:1234"
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f *fixture) status(w *httptest.ResponseRecorder, want int) {
	f.t.Helper()
	if w.Code != want {
		f.t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		f.t.Fatal("personal data cacheable")
	}
}
func (f *fixture) login(login, pass string) client {
	f.t.Helper()
	w := f.call(client{}, "POST", "/api/auth/login", map[string]any{"login": login, "password": pass})
	f.status(w, 200)
	c := client{cookies: w.Result().Cookies()}
	w = f.call(c, "GET", "/api/auth/me", nil)
	f.status(w, 200)
	var me struct {
		CSRF string `json:"csrfToken"`
	}
	if json.Unmarshal(w.Body.Bytes(), &me) != nil {
		f.t.Fatal("bad me")
	}
	c.csrf = me.CSRF
	return c
}
func (f *fixture) member(club, name string) string {
	f.t.Helper()
	p := f.id(`INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'2015-01-02') RETURNING id::text`, name)
	return f.id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value,contact_name) VALUES($1,$2,'2020-01-01','phone','synthetic-contact','Synthetic Contact') RETURNING id::text`, club, p)
}
