package demodata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/billing"
	clubapi "github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/NikishGum/JudeOSLite/backend/internal/reports"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountsAndDates(t *testing.T) {
	seen := map[string]bool{}
	names := map[string]bool{}
	accounts := Accounts()
	if len(accounts) != 922 {
		t.Fatalf("accounts: %d", len(accounts))
	}
	for _, a := range accounts {
		if seen[a.Login] {
			t.Fatalf("duplicate login: %s", a.Login)
		}
		seen[a.Login] = true
		if names[a.Name] {
			t.Fatalf("duplicate adult identity: %s", a.Name)
		}
		names[a.Name] = true
		if _, err := auth.NormalizeLogin(a.Login); err != nil {
			t.Fatal(err)
		}
		if len(strings.Fields(a.Name)) != 3 || strings.ContainsAny(a.Name, "0123456789") {
			t.Fatalf("unnatural name: %s", a.Name)
		}
	}
	for _, date := range []string{"2026-02-30", "1999-12-31", "2101-01-01", "2026-2-1"} {
		if _, err := Preview(date); err == nil {
			t.Fatalf("accepted %s", date)
		}
	}
	for _, date := range []string{"2026-10-19", "2028-02-29", "2027-01-01"} {
		r, e := Preview(date)
		if e != nil || r.From >= date || r.Until <= date {
			t.Fatalf("bad range: %+v %v", r, e)
		}
	}
}

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL demo tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("demo_%d", time.Now().UnixNano())
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
		if _, e := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH(FORCE)"); e != nil {
			t.Error(e)
		}
		admin.Close(ctx)
	})
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
func count(t *testing.T, pool *pgxpool.Pool, query string, want int, args ...any) {
	t.Helper()
	var got int
	if e := pool.QueryRow(context.Background(), query, args...).Scan(&got); e != nil {
		t.Fatal(e)
	}
	if got != want {
		t.Fatalf("%s: got %d want %d", query, got, want)
	}
}
func execute(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, e := pool.Exec(context.Background(), query, args...); e != nil {
		t.Fatal(e)
	}
}

func TestSeedIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	const password = "fictional-demo-test-password"
	// A late login conflict must roll back the whole network, preserving existing data.
	execute(t, pool, `INSERT INTO pride.clubs(name) VALUES('Существующий филиал')`)
	collision := adult(449, true, "parent")
	execute(t, pool, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,'Существующий сотрудник','untouched')`, collision.login)
	if _, e := Seed(ctx, pool, "2026-10-19", password); e == nil {
		t.Fatal("late login conflict accepted")
	}
	count(t, pool, `SELECT count(*) FROM pride.clubs`, 1)
	count(t, pool, `SELECT count(*) FROM pride.staff_users`, 1)
	count(t, pool, `SELECT count(*) FROM pride.participants`, 0)
	count(t, pool, `SELECT count(*) FROM pride.audit_events`, 0)
	execute(t, pool, `DELETE FROM pride.staff_users WHERE login=$1`, collision.login)
	// Two simultaneous requests must commit just one dataset.
	var wg sync.WaitGroup
	results := make(chan Result, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := Seed(ctx, pool, "2026-10-19", password); results <- r; failures <- e }()
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatalf("seed: %v", e)
		}
	}
	loaded := 0
	var seeded Result
	for r := range results {
		if r.AlreadyLoaded {
			loaded++
		}
		seeded = r
	}
	if loaded != 1 {
		t.Fatalf("alreadyLoaded: %d", loaded)
	}
	t.Logf("dataset: sessions=%d closed=%d", seeded.Sessions, seeded.ClosedSessions)
	for table, want := range map[string]int{"clubs": 4, "staff_users": 922, "staff_club_roles": 21, "participants": 600, "club_memberships": 600, "families": 450, "family_children": 600, "family_access": 900, "training_groups": 30, "group_memberships": 600, "schedule_rules": 30, "billing_plans": 9, "payments": 1800, "subscriptions": 1800} {
		count(t, pool, "SELECT count(*) FROM pride."+table, want)
	}
	for c := 0; c < 3; c++ {
		count(t, pool, `SELECT count(*) FROM pride.staff_club_roles WHERE club_id=$1 AND role='trainer'`, 5, id("club", c))
		count(t, pool, `SELECT count(*) FROM pride.staff_club_roles WHERE club_id=$1 AND role='administrator'`, 2, id("club", c))
		count(t, pool, `SELECT count(*) FROM pride.club_memberships WHERE club_id=$1`, 200, id("club", c))
	}
	count(t, pool, `SELECT count(*) FROM (SELECT group_id FROM pride.group_memberships GROUP BY group_id HAVING count(*)<>20) x`, 0)
	count(t, pool, `SELECT count(*) FROM pride.family_children fc JOIN pride.club_memberships m ON m.id=fc.membership_id WHERE fc.club_id<>m.club_id`, 0)
	count(t, pool, `SELECT count(*) FROM pride.participants WHERE full_name !~ '^[А-ЯЁа-яё]+ [А-ЯЁа-яё]+ [А-ЯЁа-яё]+$'`, 0)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions`, seeded.Sessions)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE status='closed'`, seeded.ClosedSessions)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE status='open'`, 3)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE status='cancelled'`, 6)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE starts_on<'2026-10-19' AND status='planned'`, 3)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE starts_on<'2026-09-07' OR starts_on>='2026-11-16'`, 0)
	count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE starts_on<>(starts_at AT TIME ZONE timezone)::date`, 0)
	count(t, pool, `SELECT count(*) FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE s.status IN ('planned','cancelled') OR (s.status='closed' AND a.state IS NULL)`, 0)
	count(t, pool, `SELECT count(*) FROM (SELECT s.id FROM pride.training_sessions s LEFT JOIN pride.attendance_records a ON a.training_session_id=s.id WHERE s.status IN ('open','closed') GROUP BY s.id HAVING count(a.id)<>20) x`, 0)
	count(t, pool, `SELECT count(DISTINCT absence_reason) FROM pride.attendance_records`, 3)
	count(t, pool, `SELECT count(*) FROM pride.attendance_records WHERE state IS NULL`, 36)
	count(t, pool, `SELECT count(DISTINCT status) FROM pride.payments`, 3)
	count(t, pool, `SELECT count(DISTINCT method) FROM pride.payments`, 4)
	count(t, pool, `SELECT count(*) FROM pride.payments WHERE currency<>'BYN' OR amount_minor NOT IN (8000,11000,14000)`, 0)
	verifyAPI(t, pool, password, seeded)
	// Rerunning with a different date/password must preserve edits and the old anchor.
	execute(t, pool, `UPDATE pride.families SET name='Семья после ручного изменения',version=version+1 WHERE id=$1`, id("family", 0))
	again, e := Seed(ctx, pool, "2026-11-01", "another-demo-password")
	if e != nil || !again.AlreadyLoaded || again.Date != "2026-10-19" {
		t.Fatalf("retry: %+v %v", again, e)
	}
	count(t, pool, `SELECT count(*) FROM pride.families WHERE name='Семья после ручного изменения'`, 1)
	var hash string
	if e = pool.QueryRow(ctx, `SELECT password_hash FROM pride.staff_users WHERE id=$1`, id("owner", 0)).Scan(&hash); e != nil || !auth.VerifyPassword(hash, password) {
		t.Fatal("retry changed password")
	}
}

type browser struct {
	cookies []*http.Cookie
	csrf    string
}

func verifyAPI(t *testing.T, pool *pgxpool.Pool, password string, result Result) {
	t.Helper()
	const origin = "https://demo.example.test"
	access, err := auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	handler := access.Handler(families.New(pool, access).Handler(billing.New(pool, access).Handler(attendance.New(pool, access).Handler(reports.New(pool, access).Handler(clubapi.New(pool, access).Handler(fallback))))))
	call := func(b browser, method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.RemoteAddr = "192.0.2.210:1234"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", b.csrf)
		for _, c := range b.cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable response")
		}
		return w
	}
	login := func(login string) browser {
		w := call(browser{}, "POST", "/api/auth/login", map[string]string{"login": login, "password": password}, 200)
		b := browser{cookies: w.Result().Cookies()}
		w = call(b, "GET", "/api/auth/me", nil, 200)
		var me struct {
			CSRF string `json:"csrfToken"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &me); e != nil {
			t.Fatal(e)
		}
		b.csrf = me.CSRF
		return b
	}
	own := login(owner().login)
	admin := login(staff(0, "administrator").login)
	trainer := login(staff(0, "trainer").login)
	parent := login(adult(0, false, "parent").login)
	base := "/api/clubs/" + id("club", 0)
	for _, b := range []browser{own, admin} {
		w := call(b, "GET", base+"/families?limit=100", nil, 200)
		var v struct {
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &v)
		if v.Total != 150 {
			t.Fatalf("family API count: %d", v.Total)
		}
	}
	call(admin, "GET", "/api/clubs/"+id("club", 1)+"/families", nil, 404)
	w := call(parent, "GET", "/api/portal/families", nil, 200)
	var fs struct {
		Items []families.Family `json:"items"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &fs); e != nil || len(fs.Items) != 1 || len(fs.Items[0].Children) != 2 {
		t.Fatalf("parent family: %s %v", w.Body.String(), e)
	}
	call(parent, "GET", base+"/member-profiles/"+id("member", 0), nil, 200)
	call(parent, "GET", base+"/member-profiles/"+id("member", 2), nil, 404)
	call(parent, "GET", "/api/portal/payments", nil, 200)
	call(parent, "GET", "/api/portal/subscriptions", nil, 200)
	call(trainer, "GET", base+"/payments", nil, 404)
	call(admin, "GET", base+"/billing/summary?from="+result.From+"&until="+result.Until, nil, 200)
	// Check the report independently against raw SQL, rather than a fixture total.
	w = call(own, "GET", "/api/reports/attendance?from="+result.From+"&until="+result.Date, nil, 200)
	var report struct {
		Totals struct {
			Present int `json:"present"`
			Absent  int `json:"absent"`
		} `json:"total"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &report); e != nil {
		t.Fatal(e)
	}
	count(t, pool, `SELECT count(*) FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE s.status='closed' AND a.state='present'`, report.Totals.Present)
	count(t, pool, `SELECT count(*) FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE s.status='closed' AND a.state='absent'`, report.Totals.Absent)
	var sid string
	if e := pool.QueryRow(context.Background(), `SELECT id::text FROM pride.training_sessions WHERE group_id=$1 AND status='open'`, id("group", 0)).Scan(&sid); e != nil {
		t.Fatal(e)
	}
	call(trainer, "GET", base+"/trainer-sessions/"+sid+"/journal", nil, 200)
	// Confirm a real pending demo invoice through the normal administrator API.
	call(admin, "POST", base+"/payments/"+id("payment", 1)+"/confirm", map[string]any{"version": 1, "method": "cash", "paidOn": result.Date}, 200)
	count(t, pool, `SELECT count(*) FROM pride.payments WHERE id=$1 AND status='paid' AND version=2`, 1, id("payment", 1))
}

func TestCalendarBoundariesIntegration(t *testing.T) {
	for _, date := range []string{"2028-02-29", "2027-01-03"} {
		t.Run(date, func(t *testing.T) {
			pool := database(t)
			r, e := Seed(context.Background(), pool, date, "calendar-demo-test-password")
			if e != nil {
				t.Fatal(e)
			}
			count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE starts_on=$1::date AND status='open'`, 3, date)
			count(t, pool, `SELECT count(*) FROM pride.training_sessions WHERE starts_on<$1::date OR starts_on>=$2::date`, 0, r.From, r.Until)
			count(t, pool, `SELECT count(*) FROM pride.training_sessions a JOIN pride.training_sessions b ON a.id<b.id AND a.trainer_id=b.trainer_id AND a.starts_at<b.ends_at AND b.starts_at<a.ends_at WHERE a.status<>'cancelled' AND b.status<>'cancelled'`, 0)
			count(t, pool, `SELECT count(*) FROM pride.training_sessions s JOIN pride.schedule_rules r ON r.id=s.schedule_rule_id WHERE s.starts_on<r.valid_from OR s.starts_on>=r.valid_until`, 0)
			count(t, pool, `SELECT count(*) FROM pride.group_memberships m JOIN pride.club_memberships c ON c.id=m.club_membership_id WHERE m.valid_from<c.joined_on`, 0)
		})
	}
}
