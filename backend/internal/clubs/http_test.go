package clubs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-clubs-password"
const origin = "https://clubs.example.test"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL clubs tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_clubs_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
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
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		admin.Close(ctx)
	})
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

type client struct {
	h       http.Handler
	cookies []*http.Cookie
	csrf    string
}

func (c client) call(method, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.RemoteAddr = "192.0.2.151:151"
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
		t.Fatal("personal response cached")
	}
}
func parse[T any](t *testing.T, w *httptest.ResponseRecorder, want int) T {
	t.Helper()
	status(t, w, want)
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func login(t *testing.T, h http.Handler, name string) client {
	t.Helper()
	c := client{h: h}
	w := c.call("POST", "/api/auth/login", map[string]string{"login": name, "password": password})
	status(t, w, 200)
	c.cookies = w.Result().Cookies()
	me := parse[struct {
		CSRF string `json:"csrfToken"`
	}](t, c.call("GET", "/api/auth/me", nil), 200)
	c.csrf = me.CSRF
	return c
}
func update(u Staff, ids []string, disabled bool) any {
	return map[string]any{"displayName": u.DisplayName, "clubIds": ids, "disabled": disabled, "version": u.Version}
}
func createAdmin(t *testing.T, c client, name string, ids []string) Staff {
	t.Helper()
	w := c.call("POST", "/api/staff", map[string]any{"login": name, "displayName": "Synthetic " + name, "password": password, "clubIds": ids})
	return parse[Staff](t, w, 201)
}
func TestManagementIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	if err := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); err != nil {
		t.Fatal(err)
	}
	access, err := auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	h := access.Handler(New(pool, access).Handler(http.NotFoundHandler()))
	owner := login(t, h, "owner")
	a := parse[Club](t, owner.call("POST", "/api/clubs", map[string]any{"name": "Hall", "address": "private hall address", "timezone": "Europe/Minsk"}), 201)
	b := parse[Club](t, owner.call("POST", "/api/clubs", map[string]any{"name": "Hall", "timezone": "UTC"}), 201)
	if a.ID == b.ID {
		t.Fatal("hall identity collapsed by name")
	}
	adminA := createAdmin(t, owner, "admin-a", []string{a.ID})
	adminB := createAdmin(t, owner, "admin-b", []string{b.ID})
	multi := createAdmin(t, owner, "multi", []string{a.ID})
	ca, cb, cm := login(t, h, adminA.Login), login(t, h, adminB.Login), login(t, h, multi.Login)
	t.Run("API boundaries and input", func(t *testing.T) {
		status(t, client{h: h}.call("GET", "/api/clubs", nil), 401)
		for _, c := range []client{ca, cb} {
			status(t, c.call("GET", "/api/clubs", nil), 403)
			status(t, c.call("GET", "/api/staff", nil), 403)
			status(t, c.call("POST", "/api/staff", map[string]any{"isOwner": true}), 403)
			status(t, c.call("PUT", "/api/staff/"+adminA.ID, update(adminA, []string{a.ID, b.ID}, false)), 403)
		}
		status(t, ca.call("GET", "/api/clubs/"+a.ID, nil), 200)
		foreign := ca.call("GET", "/api/clubs/"+b.ID, nil)
		missing := ca.call("GET", "/api/clubs/not-a-uuid", nil)
		status(t, foreign, 404)
		status(t, missing, 404)
		if foreign.Body.String() != missing.Body.String() {
			t.Fatal("foreign club existence disclosed")
		}
		status(t, ca.call("PUT", "/api/clubs/"+b.ID, map[string]any{"name": "overwrite"}), 403)
		status(t, ca.call("POST", "/api/clubs/"+a.ID+"/archive", map[string]any{"version": a.Version}), 403)
		me := ca.call("GET", "/api/auth/me", nil)
		status(t, me, 200)
		if strings.Contains(me.Body.String(), b.ID) {
			t.Fatal("foreign club leaked")
		}
		withoutCSRF := owner
		withoutCSRF.csrf = ""
		status(t, withoutCSRF.call("POST", "/api/clubs", map[string]string{"name": "No csrf", "timezone": "UTC"}), 403)
		for _, zone := range []string{"", "Local", "Not/AZone"} {
			status(t, owner.call("POST", "/api/clubs", map[string]string{"name": "invalid", "timezone": zone}), 400)
		}
		status(t, owner.call("POST", "/api/staff", map[string]any{"login": "boost", "displayName": "Boost", "password": password, "clubIds": []string{a.ID}, "isOwner": true}), 400)
		status(t, owner.call("POST", "/api/staff", map[string]any{"login": "ADMIN-A", "displayName": "Duplicate", "password": password, "clubIds": []string{b.ID}}), 409)
		status(t, owner.call("POST", "/api/staff", map[string]any{"login": "invalid", "displayName": "Bad", "password": password, "clubIds": []string{a.ID, a.ID}}), 400)
		var ownerID string
		if err := pool.QueryRow(ctx, `SELECT id::text FROM pride.staff_users WHERE is_owner`).Scan(&ownerID); err != nil {
			t.Fatal(err)
		}
		status(t, owner.call("PUT", "/api/staff/"+ownerID, map[string]any{"displayName": "Disabled Owner", "clubIds": []string{}, "disabled": true, "version": 1}), 404)
	})
	t.Run("multiple assignments immediate revocation and trainer preservation", func(t *testing.T) {
		multi = parse[Staff](t, owner.call("PUT", "/api/staff/"+multi.ID, update(multi, []string{a.ID, b.ID}, false)), 200)
		status(t, cm.call("GET", "/api/clubs/"+b.ID, nil), 200)
		// A separate trainer role is deliberately retained by administration changes.
		if _, err := pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, multi.ID, b.ID); err != nil {
			t.Fatal(err)
		}
		stale := multi
		multi = parse[Staff](t, owner.call("PUT", "/api/staff/"+multi.ID, update(multi, []string{b.ID}, false)), 200)
		status(t, cm.call("GET", "/api/clubs/"+a.ID, nil), 404)
		status(t, cm.call("GET", "/api/clubs/"+b.ID, nil), 200)
		status(t, owner.call("PUT", "/api/staff/"+multi.ID, update(stale, []string{a.ID}, false)), 409)
		multi = parse[Staff](t, owner.call("PUT", "/api/staff/"+multi.ID, update(multi, []string{}, false)), 200)
		if _, err := access.RequireClub(ctx, multi.ID, b.ID, "administrator"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("administrator role still present")
		}
		if _, err := access.RequireClub(ctx, multi.ID, b.ID, "trainer"); err != nil {
			t.Fatal("trainer role deleted", err)
		}
	})
	t.Run("disable is global and old sessions stay revoked", func(t *testing.T) {
		adminA = parse[Staff](t, owner.call("PUT", "/api/staff/"+adminA.ID, update(adminA, []string{a.ID}, true)), 200)
		status(t, ca.call("GET", "/api/auth/me", nil), 401)
		status(t, ca.call("GET", "/api/clubs/"+a.ID, nil), 401)
		status(t, client{h: h}.call("POST", "/api/auth/login", map[string]string{"login": adminA.Login, "password": password}), 401)
		adminA = parse[Staff](t, owner.call("PUT", "/api/staff/"+adminA.ID, update(adminA, []string{a.ID}, false)), 200)
		status(t, ca.call("GET", "/api/auth/me", nil), 401)
		ca = login(t, h, adminA.Login)
		status(t, ca.call("GET", "/api/clubs/"+a.ID, nil), 200)
	})
	t.Run("stale and concurrent club forms", func(t *testing.T) {
		old := a
		var wg sync.WaitGroup
		results := make(chan *httptest.ResponseRecorder, 2)
		for _, name := range []string{"Hall updated 1", "Hall updated 2"} {
			wg.Go(func() {
				results <- owner.call("PUT", "/api/clubs/"+a.ID, map[string]any{"name": name, "address": a.Address, "timezone": a.Timezone, "version": a.Version})
			})
		}
		wg.Wait()
		close(results)
		ok, conflict := 0, 0
		for w := range results {
			if w.Code == 200 {
				ok++
			} else if w.Code == 409 {
				conflict++
			} else {
				t.Fatal(w.Code, w.Body.String())
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatal("concurrent overwrite", ok, conflict)
		}
		a = parse[Club](t, owner.call("GET", "/api/clubs/"+a.ID, nil), 200)
		if a.Version != old.Version+1 {
			t.Fatal("version increment")
		}
		status(t, owner.call("POST", "/api/clubs/"+a.ID+"/archive", map[string]any{"version": old.Version}), 409)
	})
	t.Run("audit failure rolls back change", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `CREATE FUNCTION pride.reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic rejection'; END $$; CREATE TRIGGER reject_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_audit()`); err != nil {
			t.Fatal(err)
		}
		status(t, owner.call("PUT", "/api/clubs/"+a.ID, map[string]any{"name": "must rollback", "address": "", "timezone": "UTC", "version": a.Version}), 503)
		after := parse[Club](t, owner.call("GET", "/api/clubs/"+a.ID, nil), 200)
		if after != a {
			t.Fatal("write committed without audit")
		}
		if _, err := pool.Exec(ctx, `DROP TRIGGER reject_audit ON pride.audit_events; DROP FUNCTION pride.reject_audit()`); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("archive preserves rows and blocks future operations", func(t *testing.T) {
		b = parse[Club](t, owner.call("POST", "/api/clubs/"+b.ID+"/archive", map[string]any{"version": b.Version}), 200)
		if !b.Archived {
			t.Fatal("archive missing")
		}
		status(t, cb.call("GET", "/api/clubs/"+b.ID, nil), 200) // history metadata remains accessible
		me := owner.call("GET", "/api/auth/me", nil)
		status(t, me, 200)
		if strings.Contains(me.Body.String(), b.ID) {
			t.Fatal("archived club in active selector")
		}
		status(t, owner.call("PUT", "/api/staff/"+adminA.ID, update(adminA, []string{a.ID, b.ID}, false)), 409)
		// Existing archived assignments can be retained, or explicitly revoked.
		adminB = parse[Staff](t, owner.call("PUT", "/api/staff/"+adminB.ID, update(adminB, []string{b.ID}, false)), 200)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := RequireActive(ctx, tx, b.ID); !errors.Is(err, ErrArchived) {
			t.Fatal("new operation allowed in archive", err)
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2`, adminB.ID, b.ID).Scan(&count); err != nil || count != 1 {
			t.Fatal("archive deleted assignment", err)
		}
	})
	t.Run("archive serializes with an active operation", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := RequireActive(ctx, tx, a.ID); err != nil {
			t.Fatal(err)
		}
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			result <- owner.call("POST", "/api/clubs/"+a.ID+"/archive", map[string]any{"version": a.Version})
		}()
		select {
		case <-result:
			t.Fatal("archive crossed active-operation lock")
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case w := <-result:
			status(t, w, 200)
		case <-time.After(5 * time.Second):
			t.Fatal("archive deadlocked")
		}
	})
	t.Run("audit contains no secrets or contact payload", func(t *testing.T) {
		var payload string
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*),string_agg(coalesce(before_state::text,'')||coalesce(after_state::text,''),'') FROM pride.audit_events`).Scan(&count, &payload); err != nil {
			t.Fatal(err)
		}
		if count < 12 {
			t.Fatal("missing significant history", count)
		}
		for _, secret := range []string{password, "argon2", "private hall address", "Synthetic", "admin-a", "password_hash"} {
			if strings.Contains(payload, secret) {
				t.Fatal("sensitive audit payload", secret)
			}
		}
		for _, value := range []string{a.ID, b.ID, "version", "disabled", "archived"} {
			if !strings.Contains(payload, value) {
				t.Fatal("audit lost state", value)
			}
		}
		list := owner.call("GET", "/api/staff", nil)
		status(t, list, 200)
		if strings.Contains(list.Body.String(), "password") {
			t.Fatal("password exposed in staff JSON")
		}
	})
}
