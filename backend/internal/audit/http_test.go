package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testOrigin = "https://audit.example.test"
const testPassword = "synthetic-audit-password"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL audit tests")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("judeos_audit_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = name
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, e := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); e != nil {
			t.Error(e)
		}
		admin.Close(ctx)
	})
	if e = migrations.Apply(ctx, pool); e != nil {
		t.Fatal(e)
	}
	return pool
}

type client struct {
	h       http.Handler
	cookies []*http.Cookie
}

func (c client) get(path string) *httptest.ResponseRecorder { return c.call("GET", path, nil) }
func (c client) call(method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Origin", testOrigin)
	r.RemoteAddr = "192.0.2.170:170"
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("audit response cached")
	}
}
func parse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	status(t, w, 200)
	var v T
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func login(t *testing.T, h http.Handler, name string) client {
	t.Helper()
	c := client{h: h}
	w := c.call("POST", "/api/auth/login", map[string]string{"login": name, "password": testPassword})
	status(t, w, 200)
	c.cookies = w.Result().Cookies()
	return c
}
func exec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, e := p.Exec(context.Background(), sql, args...); e != nil {
		t.Fatal(e)
	}
}
func id(t *testing.T, p *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var s string
	if e := p.QueryRow(context.Background(), sql, args...).Scan(&s); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestAuditIntegration(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := auth.CreateFirstOwner(ctx, p, "owner", "Synthetic Owner", testPassword); e != nil {
		t.Fatal(e)
	}
	ownerID := id(t, p, "SELECT id::text FROM pride.staff_users WHERE is_owner")
	a := id(t, p, "INSERT INTO pride.clubs(name) VALUES('Synthetic A') RETURNING id::text")
	b := id(t, p, "INSERT INTO pride.clubs(name) VALUES('Synthetic B') RETURNING id::text")
	ids := map[string]string{}
	for _, name := range []string{"admin-a", "admin-b", "trainer", "disabled-author"} {
		ids[name] = id(t, p, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name)
	}
	for _, role := range []struct{ name, club, role string }{{"admin-a", a, "administrator"}, {"admin-b", b, "administrator"}, {"trainer", a, "trainer"}} {
		exec(t, p, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)", ids[role.name], role.club, role.role)
	}
	exec(t, p, "UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1", ids["disabled-author"])
	add := func(club any, actor, action, at string) string {
		return id(t, p, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,created_at,before_state,after_state,change_reason) VALUES($1,$2,'training_group',gen_random_uuid(),$3,$4,'{"password":"secret-password","contact":"secret-contact"}','{"token":"secret-token"}','private-reason') RETURNING id::text`, club, actor, action, at)
	}
	aID := add(a, ids["disabled-author"], "a_only", "2026-10-11T21:00:00Z")
	bID := add(b, ownerID, "b_only", "2026-10-12T20:59:59.999999Z")
	networkID := add(nil, ownerID, "network_only", "2026-10-12T12:00:00Z")
	access, e := auth.New(p, auth.Config{Origin: testOrigin})
	if e != nil {
		t.Fatal(e)
	}
	service := New(p, access)
	h := access.Handler(service.Handler(http.NotFoundHandler()))
	owner, ca, cb, trainer := login(t, h, "owner"), login(t, h, "admin-a"), login(t, h, "admin-b"), login(t, h, "trainer")
	t.Run("scope and envelope privacy", func(t *testing.T) {
		all := parse[Page](t, owner.get("/api/audit/events"))
		if len(all.Items) != 3 {
			t.Fatal(all)
		}
		for _, c := range []client{ca, cb} {
			page := parse[Page](t, c.get("/api/audit/events"))
			if len(page.Items) != 1 {
				t.Fatal(page)
			}
		}
		local := parse[Page](t, ca.get("/api/audit/events"))
		if local.Items[0].ID != aID || !local.Items[0].ActorDisabled {
			t.Fatal(local)
		}
		scoped := parse[Page](t, owner.get("/api/audit/events?clubId="+b))
		if scoped.Items[0].ID != bID {
			t.Fatal(scoped)
		}
		network := parse[Page](t, owner.get("/api/audit/events?clubId=network"))
		if len(network.Items) != 1 || network.Items[0].ID != networkID {
			t.Fatal(network)
		}
		for _, path := range []string{"/api/audit/events?clubId=" + b, "/api/audit/events?clubId=network", "/api/audit/events?clubId=00000000-0000-0000-0000-000000000000"} {
			status(t, ca.get(path), 404)
		}
		for _, path := range []string{"/api/audit/events", "/api/audit/filters"} {
			status(t, trainer.get(path), 403)
			status(t, (client{h: h}).get(path), 401)
		}
		raw := owner.get("/api/audit/events").Body.String()
		for _, secret := range []string{"secret-password", "secret-contact", "secret-token", "private-reason", "before_state", "after_state", "change_reason", "login"} {
			if strings.Contains(raw, secret) {
				t.Fatalf("exposed %q: %s", secret, raw)
			}
		}
		f := parse[Filters](t, ca.get("/api/audit/filters"))
		if f.Owner || len(f.Clubs) != 1 || f.Clubs[0].ID != a || len(f.Actions) != 1 || f.Actions[0] != "a_only" {
			t.Fatal(f)
		}
		f = parse[Filters](t, owner.get("/api/audit/filters"))
		if !f.Owner || len(f.Clubs) != 2 || len(f.Actions) != 3 {
			t.Fatal(f)
		}
	})
	t.Run("hyphenated session actions are filterable", func(t *testing.T) {
		eventID := add(a, ownerID, "replace-trainer", "2026-10-12T12:00:00Z")
		page := parse[Page](t, ca.get("/api/audit/events?action=replace-trainer"))
		if len(page.Items) != 1 || page.Items[0].ID != eventID {
			t.Fatal(page)
		}
		filters := parse[Filters](t, ca.get("/api/audit/filters"))
		if !strings.Contains(strings.Join(filters.Actions, ","), "replace-trainer") {
			t.Fatal(filters)
		}
		exec(t, p, "DELETE FROM pride.audit_events WHERE id=$1", eventID)
	})
	t.Run("inclusive Minsk days and validated filters", func(t *testing.T) {
		page := parse[Page](t, owner.get("/api/audit/events?from=2026-10-12&until=2026-10-12"))
		if len(page.Items) != 3 {
			t.Fatal(page)
		}
		page = parse[Page](t, owner.get("/api/audit/events?until=2026-10-11"))
		if len(page.Items) != 0 {
			t.Fatal(page)
		}
		page = parse[Page](t, owner.get("/api/audit/events?from=2026-10-13"))
		if len(page.Items) != 0 {
			t.Fatal(page)
		}
		page = parse[Page](t, owner.get("/api/audit/events?action=b_only"))
		if len(page.Items) != 1 || page.Items[0].ID != bID {
			t.Fatal(page)
		}
		for _, query := range []string{"from=2026-02-30", "from=0000-01-01", "from=2026-10-13&until=2026-10-12", "action=x%27", "clubId=invalid", "cursor=invalid", "action=a_only&action=b_only", "other=true"} {
			status(t, owner.get("/api/audit/events?"+query), 400)
		}
	})
	t.Run("keyset pages retain tied timestamps despite new insert", func(t *testing.T) {
		for range 55 {
			add(a, ownerID, "paged", "2026-10-12T12:00:00Z")
		}
		first := parse[Page](t, ca.get("/api/audit/events?action=paged"))
		if len(first.Items) != 50 || first.NextCursor == "" {
			t.Fatal(first)
		}
		add(a, ownerID, "paged", "2026-10-12T13:00:00Z")
		second := parse[Page](t, ca.get("/api/audit/events?action=paged&cursor="+url.QueryEscape(first.NextCursor)))
		if len(second.Items) != 5 || second.NextCursor != "" {
			t.Fatal(second)
		}
		seen := map[string]bool{}
		for _, page := range []Page{first, second} {
			for _, e := range page.Items {
				if seen[e.ID] {
					t.Fatal("duplicate", e.ID)
				}
				seen[e.ID] = true
			}
		}
		status(t, ca.get("/api/audit/events?action=a_only&cursor="+url.QueryEscape(first.NextCursor)), 400)
		refresh := parse[Page](t, ca.get("/api/audit/events?action=paged"))
		if !refresh.Items[0].CreatedAt.Equal(time.Date(2026, 10, 12, 13, 0, 0, 0, time.UTC)) {
			t.Fatal(refresh)
		}
	})
	t.Run("archive preserves audit and multiple admin assignments", func(t *testing.T) {
		exec(t, p, "UPDATE pride.clubs SET archived_at=now() WHERE id=$1", a)
		f := parse[Filters](t, ca.get("/api/audit/filters"))
		if len(f.Clubs) != 1 || !f.Clubs[0].Archived {
			t.Fatal(f)
		}
		page := parse[Page](t, ca.get("/api/audit/events?action=a_only"))
		if !page.Items[0].ClubArchived {
			t.Fatal(page)
		}
		exec(t, p, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')", ids["admin-a"], b)
		page = parse[Page](t, ca.get("/api/audit/events?action=b_only"))
		if len(page.Items) != 1 {
			t.Fatal(page)
		}
		exec(t, p, "DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2", ids["admin-a"], b)
	})
	t.Run("revocation after auth middleware is rechecked", func(t *testing.T) {
		for _, mode := range []string{"role", "session", "disable", "owner"} {
			c := ca
			if mode == "owner" {
				c = owner
			}
			wrapped := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "role":
					exec(t, p, "DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1", ids["admin-a"])
				case "session":
					exec(t, p, "UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1", ids["admin-a"])
				case "disable":
					exec(t, p, "UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1", ids["admin-a"])
				case "owner":
					exec(t, p, "UPDATE pride.staff_users SET is_owner=false WHERE id=$1", ownerID)
				}
				service.events(w, r)
			}))
			c.h = wrapped
			status(t, c.get("/api/audit/events"), 403)
			if mode == "role" {
				exec(t, p, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')", ids["admin-a"], a)
			}
			if mode == "session" {
				ca = login(t, h, "admin-a")
			}
			if mode == "disable" {
				exec(t, p, "UPDATE pride.staff_users SET disabled_at=NULL WHERE id=$1", ids["admin-a"])
				ca = login(t, h, "admin-a")
			}
			if mode == "owner" {
				exec(t, p, "UPDATE pride.staff_users SET is_owner=true WHERE id=$1", ownerID)
			}
		}
	})
}
