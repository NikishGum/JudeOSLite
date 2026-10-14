package dashboard

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
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-dashboard-password"
const origin = "https://dashboard.example.test"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL dashboard tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_dashboard_%d", time.Now().UnixNano())
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
	if err = migrations.Apply(ctx, pool); err != nil {
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
	r.RemoteAddr = "192.0.2.187:187"
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
		t.Fatal("personal dashboard response cached")
	}
}
func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	status(t, w, 200)
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func login(t *testing.T, h http.Handler, name string) client {
	t.Helper()
	c := client{h: h}
	w := c.call("POST", "/api/auth/login", map[string]string{"login": name, "password": password})
	status(t, w, 200)
	c.cookies = w.Result().Cookies()
	me := decode[struct {
		CSRF string `json:"csrfToken"`
	}](t, c.call("GET", "/api/auth/me", nil))
	c.csrf = me.CSRF
	return c
}

func TestPanelPostgresRolesDatesAndLiveChanges(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(q string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, q, args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); err != nil {
		t.Fatal(err)
	}
	access, err := auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	service := New(pool, access)
	service.now = func() time.Time { return time.Date(2000, 2, 1, 22, 30, 0, 0, time.UTC) }
	h := access.Handler(service.Handler(attendance.New(pool, access).Handler(sessions.New(pool, access).Handler(http.NotFoundHandler()))))
	owner := login(t, h, "owner")
	a := id(`INSERT INTO pride.clubs(name,timezone) VALUES('A','Europe/Minsk') RETURNING id::text`)
	b := id(`INSERT INTO pride.clubs(name,timezone) VALUES('B','UTC') RETURNING id::text`)
	ga := id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Same name') RETURNING id::text`, a)
	gb := id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Same name') RETURNING id::text`, b)
	id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Empty unassigned') RETURNING id::text`, a)
	staff := func(name, club, role string) client {
		uid := id(`INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name)
		exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, uid, club, role)
		return login(t, h, name)
	}
	ta, tb := staff("trainer-a", a, "trainer"), staff("trainer-b", b, "trainer")
	admin := staff("administrator", a, "administrator")
	members := func(club, group string, n int) []string {
		ids := []string{}
		for i := 0; i < n; i++ {
			p := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'1990-01-01') RETURNING id::text`, fmt.Sprintf("Synthetic %d", i))
			m := id(`INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'1999-01-01','phone','private-contact') RETURNING id::text`, p, club)
			exec(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'1999-01-01')`, club, group, m)
			ids = append(ids, m)
		}
		return ids
	}
	ma := members(a, ga, 5)
	members(b, gb, 2)
	// A full registry is deliberately larger than the paginated participants API.
	exec(`WITH people AS (INSERT INTO pride.participants(full_name,birth_date) SELECT 'Additional '||i,'1990-01-01'::date FROM generate_series(1,105)i RETURNING id) INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) SELECT id,$1,'1999-01-01','phone','private-contact' FROM people`, a)
	newSession := func(club, group, trainer, stamp, date, zone string) string {
		return id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) SELECT $1,$2,id,$4::date,$5,$3::timestamptz,$3::timestamptz+interval '1 hour' FROM pride.staff_users WHERE login=$6 RETURNING id::text`, club, group, stamp, date, zone, trainer)
	}
	path := func(club, sid string) string { return "/api/clubs/" + club + "/trainer-sessions/" + sid + "/journal" }
	open := func(c client, club, sid string) attendance.Journal {
		return decode[attendance.Journal](t, c.call("POST", path(club, sid)+"/open", map[string]any{"version": 1}))
	}
	old := newSession(a, ga, "trainer-a", "2000-02-01T18:00:00Z", "2000-02-01", "Europe/Minsk")
	jo := open(ta, a, old)
	jo = decode[attendance.Journal](t, ta.call("PUT", path(a, old)+"/records/"+jo.Records[0].ID, map[string]any{"version": jo.Records[0].Version, "sessionVersion": jo.Session.Version, "state": "present", "absenceReason": nil}))
	newSession(a, ga, "trainer-a", "2000-02-01T23:00:00Z", "2000-02-02", "Europe/Minsk")
	closed := newSession(a, ga, "trainer-a", "2000-02-01T22:05:00Z", "2000-02-02", "Europe/Minsk")
	jc := open(ta, a, closed)
	decode[attendance.Journal](t, ta.call("POST", path(a, closed)+"/close", map[string]any{"version": jc.Session.Version}))
	cancelled := newSession(a, ga, "trainer-a", "2000-02-02T01:00:00Z", "2000-02-02", "Europe/Minsk")
	status(t, owner.call("POST", "/api/clubs/"+a+"/sessions/"+cancelled+"/cancel", map[string]any{"version": 1, "reason": "Synthetic cancellation"}), 200)
	newSession(a, ga, "trainer-a", "2000-02-02T23:00:00Z", "2000-02-03", "Europe/Minsk")
	bo := newSession(b, gb, "trainer-b", "2000-02-01T23:00:00Z", "2000-02-01", "UTC")
	open(tb, b, bo)
	newSession(b, gb, "trainer-b", "2000-02-02T02:00:00Z", "2000-02-02", "UTC")
	exec(`UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, ma[4])
	get := func(c client, club string) Panel {
		return decode[Panel](t, c.call("GET", "/api/dashboard?clubId="+club, nil))
	}
	v := get(owner, "")
	if v.Mode != "owner" || len(v.Clubs) != 2 || v.Counters.Participants == nil || *v.Counters.Participants != 111 || v.Counters.Groups != 3 || v.Counters.Today != 4 || v.Counters.Open != 2 {
		t.Fatalf("network %+v", v)
	}
	if v.Clubs[0].Today != "2000-02-02" || v.Clubs[1].Today != "2000-02-01" {
		t.Fatalf("local dates %+v", v.Clubs)
	}
	if v.Next == nil || v.Next.ID != old || v.Next.Status != "open" || v.Next.Roster != 5 || v.Next.Marked != 1 || v.Next.CanMark {
		t.Fatalf("next open/frozen snapshot %+v", v.Next)
	}
	va := get(admin, a)
	if va.Mode != "administrator" || len(va.Clubs) != 1 || *va.Counters.Participants != 109 || va.Counters.Today != 3 || va.Next.CanMark {
		t.Fatalf("administrator %+v", va)
	}
	vt := get(ta, "")
	if vt.Mode != "trainer" || vt.Counters.Participants != nil || vt.Counters.Groups != 1 || vt.Counters.Today != 3 || vt.Counters.Open != 1 || !vt.Next.CanMark || len(vt.Groups) != 1 || vt.Groups[0].Members != 4 {
		t.Fatalf("trainer %+v", vt)
	}
	status(t, admin.call("GET", "/api/dashboard?clubId="+b, nil), 404)
	status(t, ta.call("GET", "/api/dashboard?clubId="+b, nil), 404)
	status(t, owner.call("GET", "/api/dashboard?clubId=bad", nil), 400)
	status(t, (client{h: h}).call("GET", "/api/dashboard", nil), 401)
	raw := owner.call("GET", "/api/dashboard", nil).Body.String()
	for _, private := range []string{"private-contact", "birthDate", "csrfToken", "records", "password", "Synthetic 0"} {
		if strings.Contains(raw, private) {
			t.Fatalf("private field leaked: %s", private)
		}
	}
	// The branch's current calendar never rewrites a session's saved timezone.
	exec(`UPDATE pride.clubs SET timezone='UTC' WHERE id=$1`, a)
	shifted := get(owner, a)
	if shifted.Clubs[0].Today != "2000-02-01" || shifted.Clubs[0].Timezone != "UTC" {
		t.Fatalf("branch clock %+v", shifted.Clubs)
	}
	differentDate := false
	for _, session := range shifted.Today {
		if session.Timezone == "Europe/Minsk" && session.Date == "2000-02-02" && session.BranchDate == "2000-02-01" {
			differentDate = true
		}
	}
	if !differentDate {
		t.Fatal("saved session clock was replaced by current branch clock")
	}
	exec(`UPDATE pride.clubs SET timezone='Europe/Minsk' WHERE id=$1`, a)
	// Establish the read snapshot, then commit a real mark before the domain
	// SELECTs. A panel must use one snapshot, followed by fresh values on retry.
	fixedNow := service.now
	begun, resume := make(chan struct{}), make(chan struct{})
	service.now = func() time.Time { close(begun); <-resume; return fixedNow() }
	completed := make(chan *httptest.ResponseRecorder, 1)
	go func() { completed <- owner.call("GET", "/api/dashboard?clubId="+a, nil) }()
	<-begun
	jo = decode[attendance.Journal](t, ta.call("PUT", path(a, old)+"/records/"+jo.Records[1].ID, map[string]any{"version": jo.Records[1].Version, "sessionVersion": jo.Session.Version, "state": "absent", "absenceReason": "illness"}))
	close(resume)
	snapshot := decode[Panel](t, <-completed)
	service.now = fixedNow
	if snapshot.Next.Marked != 1 || snapshot.Unfinished[0].Marked != 1 {
		t.Fatal("panel mixed two database snapshots")
	}
	if get(ta, a).Next.Marked != 2 {
		t.Fatal("next read did not observe committed mark")
	}
	// Role priority comes from live assignments, and trainer actions from actual ID.
	exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) SELECT id,$1,'trainer' FROM pride.staff_users WHERE login='administrator'`, b)
	if multi := get(admin, ""); multi.Mode != "administrator" || len(multi.Clubs) != 1 || multi.Clubs[0].ID != a {
		t.Fatalf("multi-role priority %+v", multi)
	}
	if multi := get(admin, b); multi.Mode != "trainer" || multi.Counters.Participants != nil || multi.Next != nil {
		t.Fatalf("multi-role selected trainer scope %+v", multi)
	}
	// Mark/close must be visible immediately; closed/cancelled never become next.
	jo = decode[attendance.Journal](t, ta.call("PUT", path(a, old)+"/records/"+jo.Records[1].ID, map[string]any{"version": jo.Records[1].Version, "sessionVersion": jo.Session.Version, "state": "absent", "absenceReason": "illness"}))
	if get(ta, a).Next.Marked != 2 {
		t.Fatal("mark not refreshed")
	}
	decode[attendance.Journal](t, ta.call("POST", path(a, old)+"/close", map[string]any{"version": jo.Session.Version}))
	if next := get(ta, a); next.Counters.Open != 0 || next.Next == nil || next.Next.Status != "planned" {
		t.Fatalf("next after close %+v", next)
	}
	exec(`UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, ga)
	if archived := get(ta, a); archived.Counters.Groups != 0 || len(archived.Groups) != 0 || archived.Counters.Today != 3 {
		t.Fatalf("archived group changed historical sessions %+v", archived)
	}
	// Authenticate first, then revoke. A stale middleware principal grants nothing.
	paused := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='trainer-a')`)
		service.read(w, r)
	}))
	live := ta
	live.h = paused
	status(t, live.call("GET", "/api/dashboard?clubId="+a, nil), 404)
	if empty := get(ta, ""); len(empty.Clubs) != 0 || empty.Next != nil || empty.Counters.Participants != nil {
		t.Fatalf("revoked stale data %+v", empty)
	}
	exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, b)
	status(t, owner.call("GET", "/api/dashboard?clubId="+b, nil), 404)
	for _, mutation := range []string{`UPDATE pride.staff_users SET disabled_at=now() WHERE login='owner'`, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='owner')`} {
		exec(`UPDATE pride.staff_users SET disabled_at=NULL WHERE login='owner'`)
		live := login(t, h, "owner")
		live.h = access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { exec(mutation); service.read(w, r) }))
		status(t, live.call("GET", "/api/dashboard", nil), 404)
	}
}
