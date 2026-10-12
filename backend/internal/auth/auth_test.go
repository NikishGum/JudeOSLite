package auth

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

	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testPassword = "synthetic-test-password"
const testOrigin = "https://crm.example.test"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required for real PostgreSQL access tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_auth_test_%d", time.Now().UnixNano())
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
		_, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)")
		if err != nil {
			t.Error(err)
		}
		admin.Close(ctx)
	})
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
func execSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
func queryID(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func call(handler http.Handler, method, path, origin, csrf string, cookies []*http.Cookie, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status %d, want %d: %s", w.Code, want, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("personal response may be cached")
	}
}
func loginAs(t *testing.T, h http.Handler, login string) ([]*http.Cookie, string) {
	t.Helper()
	w := call(h, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": login, "password": testPassword})
	status(t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatal("session/CSRF cookie pair missing")
	}
	for _, c := range cookies {
		if !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteStrictMode || c.MaxAge != 43200 || !strings.HasPrefix(c.Name, "__Host-") {
			t.Fatal("unsafe session cookie")
		}
	}
	me := call(h, "GET", "/api/auth/me", "", "", cookies, nil)
	status(t, me, 200)
	var body struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !validToken(body.CSRF) {
		t.Fatal("missing CSRF token")
	}
	return cookies, body.CSRF
}
func TestAccessIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	// Two concurrent bootstrap requests can create only one first owner.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { results <- CreateFirstOwner(ctx, pool, " Owner ", "Test Owner", testPassword) })
	}
	wg.Wait()
	close(results)
	successful := 0
	for err := range results {
		if err == nil {
			successful++
		}
	}
	if successful != 1 {
		t.Fatalf("bootstrap winners: %d", successful)
	}
	owner := queryID(t, pool, "SELECT id::text FROM pride.staff_users WHERE login='owner'")
	a := queryID(t, pool, "INSERT INTO pride.clubs(name) VALUES('Club A') RETURNING id::text")
	b := queryID(t, pool, "INSERT INTO pride.clubs(name) VALUES('Club B') RETURNING id::text")
	hash, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, login := range []string{"admin", "trainer", "multi", "disabled", "rotation", "expired", "revoked"} {
		execSQL(t, pool, "INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,$1,$2)", login, hash)
	}
	admin := queryID(t, pool, "SELECT id::text FROM pride.staff_users WHERE login='admin'")
	trainer := queryID(t, pool, "SELECT id::text FROM pride.staff_users WHERE login='trainer'")
	execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role)
        SELECT id,$1::uuid,'administrator' FROM pride.staff_users WHERE login IN ('admin','multi')`, a)
	execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role)
        SELECT id,$1::uuid,'trainer' FROM pride.staff_users WHERE login IN ('trainer','multi')`, b)
	execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) SELECT id,$1::uuid,'trainer' FROM pride.staff_users WHERE login='multi'`, a)
	s, err := New(pool, Config{Origin: testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler(http.NotFoundHandler())
	t.Run("credentials and login CSRF", func(t *testing.T) {
		input := map[string]string{"login": "owner", "password": testPassword}
		for _, origin := range []string{"", "https://foreign.example", "null"} {
			status(t, call(h, "POST", "/api/auth/login", origin, "", nil, input), 403)
		}
		for _, login := range []string{"owner", "unknown"} {
			w := call(h, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": login, "password": "wrong-password"})
			status(t, w, 401)
			if !strings.Contains(w.Body.String(), "invalid_credentials") {
				t.Fatal("credential enumeration")
			}
		}
		status(t, call(h, "GET", "/api/auth/me", "", "", nil, nil), 401)
		status(t, call(h, "GET", "/api/auth/me", "", "", []*http.Cookie{{Name: "__Host-pride_session", Value: "forged"}}, nil), 401)
	})
	t.Run("owner sees network and secrets stay hashed", func(t *testing.T) {
		cookies, _ := loginAs(t, h, "owner")
		w := call(h, "GET", "/api/auth/me", "", "", cookies, nil)
		status(t, w, 200)
		if !strings.Contains(w.Body.String(), a) || !strings.Contains(w.Body.String(), b) {
			t.Fatal("owner network incomplete")
		}
		for _, c := range cookies {
			if strings.Contains(w.Body.String(), c.Value) && c.Name == "__Host-pride_session" {
				t.Fatal("session token leaked in JSON")
			}
		}
		if strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "argon2") {
			t.Fatal("password data leaked")
		}
		var tokenHash, csrfHash []byte
		if err := pool.QueryRow(ctx, "SELECT token_hash,csrf_token_hash FROM pride.staff_sessions WHERE staff_user_id=$1", owner).Scan(&tokenHash, &csrfHash); err != nil {
			t.Fatal(err)
		}
		for _, c := range cookies {
			want := tokenHash
			if c.Name == "__Host-pride_csrf" {
				want = csrfHash
			}
			if !bytes.Equal(want, digest(c.Value)) || bytes.Contains(want, []byte(c.Value)) {
				t.Fatal("session storage is not hashed")
			}
		}
		for _, id := range []string{a, b} {
			status(t, call(h, "GET", "/api/auth/clubs/"+id, "", "", cookies, nil), 200)
		}
	})
	t.Run("club boundaries and live roles", func(t *testing.T) {
		cookies, _ := loginAs(t, h, "admin")
		w := call(h, "GET", "/api/auth/me", "", "", cookies, nil)
		status(t, w, 200)
		if strings.Contains(w.Body.String(), b) || !strings.Contains(w.Body.String(), a) {
			t.Fatal("admin club leak")
		}
		status(t, call(h, "GET", "/api/auth/clubs/"+a, "", "", cookies, nil), 200)
		foreign := call(h, "GET", "/api/auth/clubs/"+b, "", "", cookies, nil)
		status(t, foreign, 404)
		missing := call(h, "GET", "/api/auth/clubs/not-a-uuid", "", "", cookies, nil)
		status(t, missing, 404)
		if foreign.Body.String() != missing.Body.String() {
			t.Fatal("foreign ID disclosure")
		}
		if _, err := s.RequireClub(ctx, trainer, b, "administrator"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("trainer got admin access")
		}
		if _, err := s.RequireClub(ctx, owner, b, "administrator"); err != nil {
			t.Fatal(err)
		}
		execSQL(t, pool, "DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1", admin)
		status(t, call(h, "GET", "/api/auth/clubs/"+a, "", "", cookies, nil), 404)
		w = call(h, "GET", "/api/auth/me", "", "", cookies, nil)
		status(t, w, 200)
		if strings.Contains(w.Body.String(), a) {
			t.Fatal("role revocation not immediate")
		}
		multi, _ := loginAs(t, h, "multi")
		w = call(h, "GET", "/api/auth/me", "", "", multi, nil)
		status(t, w, 200)
		if !strings.Contains(w.Body.String(), a) || !strings.Contains(w.Body.String(), b) || !strings.Contains(w.Body.String(), `["administrator","trainer"]`) {
			t.Fatal("multiple assignments missing")
		}
	})
	t.Run("CSRF, logout and rotation", func(t *testing.T) {
		cookies, csrf := loginAs(t, h, "rotation")
		for _, bad := range []string{"", randomToken()} {
			status(t, call(h, "POST", "/api/auth/logout", testOrigin, bad, cookies, nil), 403)
		}
		status(t, call(h, "POST", "/api/auth/logout", "https://foreign.example", csrf, cookies, nil), 403)
		status(t, call(h, "GET", "/api/auth/me", "", "", cookies, nil), 200)
		w := call(h, "POST", "/api/auth/login", testOrigin, "", cookies, map[string]string{"login": "rotation", "password": testPassword})
		status(t, w, 200)
		rotated := w.Result().Cookies()
		status(t, call(h, "GET", "/api/auth/me", "", "", cookies, nil), 401)
		me := call(h, "GET", "/api/auth/me", "", "", rotated, nil)
		status(t, me, 200)
		var body struct {
			CSRF string `json:"csrfToken"`
		}
		json.Unmarshal(me.Body.Bytes(), &body)
		w = call(h, "POST", "/api/auth/logout", testOrigin, body.CSRF, rotated, nil)
		status(t, w, 200)
		for _, c := range w.Result().Cookies() {
			if c.MaxAge != -1 {
				t.Fatal("logout did not delete cookie")
			}
		}
		status(t, call(h, "GET", "/api/auth/me", "", "", rotated, nil), 401)
	})
	t.Run("expiry revocation and disable", func(t *testing.T) {
		expired, _ := loginAs(t, h, "expired")
		execSQL(t, pool, "UPDATE pride.staff_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='expired')")
		status(t, call(h, "GET", "/api/auth/me", "", "", expired, nil), 401)
		revoked, _ := loginAs(t, h, "revoked")
		execSQL(t, pool, "UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='revoked')")
		status(t, call(h, "GET", "/api/auth/me", "", "", revoked, nil), 401)
		disabled, _ := loginAs(t, h, "disabled")
		execSQL(t, pool, "UPDATE pride.staff_users SET disabled_at=now() WHERE login='disabled'")
		status(t, call(h, "GET", "/api/auth/me", "", "", disabled, nil), 401)
		status(t, call(h, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": "disabled", "password": testPassword}), 401)
		execSQL(t, pool, "UPDATE pride.staff_users SET disabled_at=NULL WHERE login='disabled'")
		status(t, call(h, "GET", "/api/auth/me", "", "", disabled, nil), 401)
	})
	t.Run("login throttling", func(t *testing.T) {
		// Fresh service isolates the limiter's fixed window from previous login scenarios.
		fresh, err := New(pool, Config{Origin: testOrigin})
		if err != nil {
			t.Fatal(err)
		}
		handler := fresh.Handler(http.NotFoundHandler())
		for range 5 {
			status(t, call(handler, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": "admin", "password": "wrong-password"}), 401)
		}
		w := call(handler, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": "admin", "password": testPassword})
		status(t, w, 429)
		if w.Header().Get("Retry-After") == "" {
			t.Fatal("retry interval missing")
		}
	})
}

func TestPasswordAndConfig(t *testing.T) {
	hash, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, testPassword) || VerifyPassword(hash, "wrong") {
		t.Fatal("password verification")
	}
	for _, hash := range []string{"plaintext", strings.Replace(hash, "m=65536", "m=999999999", 1), hash + "$extra"} {
		if VerifyPassword(hash, testPassword) {
			t.Fatal("invalid hash accepted")
		}
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	for _, cfg := range []Config{{Origin: "http://localhost:8080"}, {Origin: "http://public.example", InsecureLocal: true}, {Origin: "https://example.test/"}, {Origin: "https://user@example.test"}} {
		if cfg.Validate() == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
	for _, cfg := range []Config{{Origin: testOrigin}, {Origin: "http://localhost:8080", InsecureLocal: true}} {
		if cfg.Validate() != nil {
			t.Fatal("valid origin rejected")
		}
	}
}
func TestLimiterWindowAndIP(t *testing.T) {
	l := limiter{entries: map[string]attempts{}}
	now := time.Now()
	for range 5 {
		if !l.allow("ip1", "login1", now) {
			t.Fatal("early limit")
		}
	}
	if l.allow("ip2", "login1", now) {
		t.Fatal("distributed login bypass")
	}
	if !l.allow("ip1", "login1", now.Add(15*time.Minute)) {
		t.Fatal("window did not expire")
	}
	l = limiter{entries: map[string]attempts{}}
	for i := range 20 {
		if !l.allow("ip1", fmt.Sprint(i), now) {
			t.Fatal("early IP limit")
		}
	}
	if l.allow("ip1", "new-login", now) {
		t.Fatal("IP limit bypass")
	}
}
