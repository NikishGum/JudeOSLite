package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/corrections"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const password = "synthetic-sessions-password"
const origin = "https://sessions.example.test"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL attendance tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_attendance_%d", time.Now().UnixNano())
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
func decode[T any](t *testing.T, w *httptest.ResponseRecorder, want int) T {
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
	me := decode[struct {
		CSRF string `json:"csrfToken"`
	}](t, c.call("GET", "/api/auth/me", nil), 200)
	c.csrf = me.CSRF
	return c
}

func TestAttendanceReportIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	id := func(q string, args ...any) string {
		t.Helper()
		var v string
		if e := pool.QueryRow(ctx, q, args...).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if e := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); e != nil {
		t.Fatal(e)
	}
	access, e := auth.New(pool, auth.Config{Origin: origin})
	if e != nil {
		t.Fatal(e)
	}
	service := New(pool, access)
	h := access.Handler(service.Handler(corrections.New(pool, access).Handler(attendance.New(pool, access).Handler(sessions.New(pool, access).Handler(http.NotFoundHandler())))))
	owner := login(t, h, "owner")
	staff := func(name, club, role string) client {
		uid := id(`INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name)
		exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, uid, club, role)
		return login(t, h, name)
	}
	a := id(`INSERT INTO pride.clubs(name,timezone) VALUES('A','Europe/Minsk') RETURNING id::text`)
	b := id(`INSERT INTO pride.clubs(name,timezone) VALUES('B','UTC') RETURNING id::text`)
	ga := id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Same name') RETURNING id::text`, a)
	gb := id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Same name') RETURNING id::text`, b)
	empty := id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Empty') RETURNING id::text`, a)
	ta, tb := staff("trainer-a", a, "trainer"), staff("trainer-b", b, "trainer")
	admin := staff("admin", a, "administrator")
	parentID := id(`INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT 'parent','Synthetic Parent',password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`)
	familyID := id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Synthetic family') RETURNING id::text`, a)
	exec(`INSERT INTO pride.family_access(family_id,user_id) SELECT $1,id FROM pride.staff_users WHERE id=$2 OR login IN ('owner','admin','trainer-a')`, familyID, parentID)
	parent := login(t, h, "parent")
	// Multiple administrator/trainer roles do not grant owner statistics.
	exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) SELECT id,$1,'administrator' FROM pride.staff_users WHERE login='admin'`, b)
	member := func(club, group string, n int) {
		for i := 0; i < n; i++ {
			p := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'2015-01-01') RETURNING id::text`, fmt.Sprintf("Synthetic %s %d", club, i))
			m := id(`INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'1999-01-01','phone','private-contact') RETURNING id::text`, p, club)
			exec(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'1999-01-01')`, club, group, m)
		}
	}
	member(a, ga, 4)
	member(b, gb, 3)
	newSession := func(club, group, trainer, stamp, sourceDate, zone string) string {
		return id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) SELECT $1,$2,id,$4::date,$5,$3::timestamptz,$3::timestamptz+interval '1 hour' FROM pride.staff_users WHERE login=$6 RETURNING id::text`, club, group, stamp, sourceDate, zone, trainer)
	}
	path := func(club, sid string) string { return "/api/clubs/" + club + "/trainer-sessions/" + sid + "/journal" }
	open := func(c client, club, sid string) attendance.Journal {
		return decode[attendance.Journal](t, c.call("POST", path(club, sid)+"/open", map[string]any{"version": 1}), 200)
	}
	mark := func(c client, club string, j attendance.Journal, i int, state string, reason any) attendance.Journal {
		return decode[attendance.Journal](t, c.call("PUT", path(club, j.Session.ID)+"/records/"+j.Records[i].ID, map[string]any{"version": j.Records[i].Version, "sessionVersion": j.Session.Version, "state": state, "absenceReason": reason}), 200)
	}
	closeJournal := func(c client, club string, j attendance.Journal) attendance.Journal {
		return decode[attendance.Journal](t, c.call("POST", path(club, j.Session.ID)+"/close", map[string]any{"version": j.Session.Version}), 200)
	}
	// Actual local date is February 1; UTC/source date is January 31.
	sa := newSession(a, ga, "trainer-a", "2000-02-01 00:30+02", "2000-01-31", "Europe/Minsk")
	ja := open(ta, a, sa)
	ja = mark(ta, a, ja, 0, "present", nil)
	ja = mark(ta, a, ja, 1, "present", nil)
	ja = mark(ta, a, ja, 2, "absent", "illness")
	ja = closeJournal(ta, a, ja)
	sb := newSession(b, gb, "trainer-b", "2000-02-01 23:59+00", "2000-02-01", "UTC")
	jb := open(tb, b, sb)
	jb = mark(tb, b, jb, 0, "present", nil)
	jb = mark(tb, b, jb, 1, "absent", "illness")
	jb = mark(tb, b, jb, 2, "absent", "excused")
	jb = closeJournal(tb, b, jb)
	se := newSession(a, empty, "trainer-a", "2000-02-01 19:00+03", "2000-02-01", "Europe/Minsk")
	closeJournal(ta, a, open(ta, a, se))
	so := newSession(a, ga, "trainer-a", "2000-02-01 18:00+03", "2000-02-01", "Europe/Minsk")
	jo := open(ta, a, so)
	mark(ta, a, jo, 0, "present", nil)
	newSession(b, gb, "trainer-b", "2000-02-01 15:00+00", "2000-02-01", "UTC")
	sc := newSession(a, ga, "trainer-a", "2000-02-01 20:00+03", "2000-02-01", "Europe/Minsk")
	jc := open(ta, a, sc)
	jc = mark(ta, a, jc, 0, "absent", "illness")
	status(t, owner.call("POST", "/api/clubs/"+a+"/sessions/"+sc+"/cancel", map[string]any{"version": jc.Session.Version, "reason": "synthetic cancel", "confirmOpen": true}), 200)
	retired := newSession(b, gb, "trainer-b", "2000-02-01 16:00+00", "2000-02-01", "UTC")
	exec(`UPDATE pride.training_sessions SET superseded_at=now() WHERE id=$1`, retired)
	outside := newSession(a, empty, "trainer-a", "2000-02-02 00:00+02", "2000-02-02", "Europe/Minsk")
	closeJournal(ta, a, open(ta, a, outside))
	reportPath := "/api/reports/attendance?from=2000-02-01&until=2000-02-01"
	get := func(extra string) Report { return decode[Report](t, owner.call("GET", reportPath+extra, nil), 200) }
	assertTotals := func(v Totals, closed, planned, opened, present, absent, unknown, illness, excused int64) {
		t.Helper()
		if v.Closed != closed || v.Planned != planned || v.Open != opened || v.Present != present || v.Absent != absent || v.Unknown != unknown || v.Illness != illness || v.Excused != excused {
			t.Fatalf("wrong totals %+v", v)
		}
		if v.Absent != v.Unknown+v.Illness+v.Excused {
			t.Fatal("reason sum")
		}
	}
	r := get("")
	assertTotals(r.Total, 3, 1, 1, 3, 4, 1, 2, 1)
	if len(r.OpenSessions) != int(r.Total.Open) || r.OpenSessions[0].ID != so || r.OpenSessions[0].ClubID != a || r.OpenSessions[0].GroupID != ga || r.OpenSessions[0].Date != "2000-02-01" || r.OpenSessions[0].Timezone != "Europe/Minsk" {
		t.Fatalf("open list differs from same-snapshot counter: %+v", r.OpenSessions)
	}
	if r.Total.Percentage == nil || math.Abs(*r.Total.Percentage-300.0/7) > 1e-9 {
		t.Fatalf("weighted percentage %+v", r.Total)
	}
	if len(r.Clubs) != 2 || len(r.Groups) != 3 {
		t.Fatalf("levels %+v", r)
	}
	filtered := get("&clubId=" + a)
	assertTotals(filtered.Total, 2, 0, 1, 2, 2, 1, 1, 0)
	if len(filtered.OpenSessions) != 1 || filtered.OpenSessions[0].ID != so {
		t.Fatal("open list club scope")
	}
	filtered = get("&groupId=" + gb)
	assertTotals(filtered.Total, 1, 1, 0, 1, 2, 0, 1, 1)
	if len(filtered.OpenSessions) != 0 {
		t.Fatal("open list leaked another group")
	}
	if len(filtered.Clubs) != 1 || *filtered.Clubs[0].ClubID != b || len(filtered.Groups) != 1 {
		t.Fatal("group ID scope")
	}
	filtered = get("&clubId=" + a + "&groupId=" + empty)
	assertTotals(filtered.Total, 1, 0, 0, 0, 0, 0, 0, 0)
	if filtered.Total.Percentage != nil {
		t.Fatal("zero denominator must be null")
	}
	// Transfer/archive after freeze does not move historical marks or remove them.
	moved := ja.Records[0].MembershipID
	exec(`UPDATE pride.group_memberships SET valid_until='2000-02-02' WHERE club_membership_id=$1`, moved)
	exec(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2000-02-02')`, a, empty, moved)
	exec(`UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, moved)
	exec(`UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, ga)
	assertTotals(get("").Total, 3, 1, 1, 3, 4, 1, 2, 1)
	// A real closed correction changes current sums, while history remains.
	rid := ja.Records[2].ID
	status(t, owner.call("PUT", "/api/clubs/"+a+"/journals/"+sa+"/records/"+rid, map[string]any{"version": ja.Records[2].Version, "state": "present", "absenceReason": nil, "changeReason": "Synthetic correction"}), 200)
	r = get("")
	assertTotals(r.Total, 3, 1, 1, 4, 3, 1, 1, 1)
	if *r.Total.Percentage != 400.0/7 {
		t.Fatal("correction percentage")
	}
	exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, a)
	assertTotals(get("").Total, 3, 1, 1, 4, 3, 1, 1, 1)
	if len(get("").OpenSessions) != 1 {
		t.Fatal("archive lost historical open session")
	}
	f := decode[Filters](t, owner.call("GET", "/api/reports/filters", nil), 200)
	if len(f.Clubs) != 2 || len(f.Groups) != 3 {
		t.Fatal("historical filters")
	}
	raw := owner.call("GET", reportPath, nil).Body.String()
	for _, private := range []string{"private-contact", "birthDate", "membershipId", "Synthetic correction", "Synthetic Owner", "markedBy", "login", "password"} {
		if strings.Contains(raw, private) {
			t.Fatalf("privacy: %s", private)
		}
	}
	for _, c := range []client{parent, admin, ta, tb} {
		status(t, c.call("GET", reportPath, nil), 403)
		status(t, c.call("GET", "/api/reports/filters", nil), 403)
	}
	status(t, (client{h: h}).call("GET", reportPath, nil), 401)
	for _, suffix := range []string{"&clubId=invalid", "&groupId=invalid", "&extra=true", "&from=2000-02-01"} {
		status(t, owner.call("GET", reportPath+suffix, nil), 400)
	}
	for _, dates := range []string{"from=2000-02-30&until=2000-03-01", "from=0000-01-01&until=2000-01-01", "from=2000-02-02&until=2000-02-01", "from=2000-02-01", "from=2000-2-1&until=2000-02-01"} {
		status(t, owner.call("GET", "/api/reports/attendance?"+dates, nil), 400)
	}
	status(t, owner.call("GET", reportPath+"&clubId="+b+"&groupId="+ga, nil), 404)
	status(t, owner.call("GET", reportPath+"&clubId=00000000-0000-0000-0000-000000000000", nil), 404)
	zero := decode[Report](t, owner.call("GET", "/api/reports/attendance?from=9999-12-31&until=9999-12-31", nil), 200)
	assertTotals(zero.Total, 0, 0, 0, 0, 0, 0, 0, 0)
	if len(zero.OpenSessions) != 0 {
		t.Fatal("open list outside period")
	}
	if zero.Total.Percentage != nil {
		t.Fatal("empty period percentage")
	}
	noGroups := id(`INSERT INTO pride.clubs(name,timezone) VALUES('No groups','UTC') RETURNING id::text`)
	zero = get("&clubId=" + noGroups)
	if len(zero.Clubs) != 1 || len(zero.Groups) != 0 || zero.Total.Percentage != nil {
		t.Fatal("club without groups")
	}
	// Snapshot-consistent totals while a correction is committed concurrently.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 8; i++ {
			value := "absent"
			var reason any = "excused"
			if i%2 != 0 {
				value = "present"
				reason = nil
			}
			j := decode[corrections.Journal](t, owner.call("GET", "/api/clubs/"+b+"/journals/"+sb, nil), 200)
			status(t, owner.call("PUT", "/api/clubs/"+b+"/journals/"+sb+"/records/"+jb.Records[0].ID, map[string]any{"version": j.Records[0].Version, "state": value, "absenceReason": reason, "changeReason": "Concurrent report correction"}), 200)
		}
	}()
	for i := 0; i < 20; i++ {
		v := get("")
		if len(v.OpenSessions) != int(v.Total.Open) {
			t.Fatal("mixed open-session snapshot")
		}
		var p, a int64
		for _, row := range v.Groups {
			p += row.Present
			a += row.Absent
		}
		if p != v.Total.Present || a != v.Total.Absent {
			t.Fatal("mixed snapshots")
		}
	}
	<-done
	// Revoke permissions/session after authentication middleware, before SQL read.
	for _, mutation := range []string{`UPDATE pride.staff_users SET is_owner=false WHERE login='owner'`, `UPDATE pride.staff_users SET disabled_at=now() WHERE login='owner'`, `UPDATE pride.staff_sessions SET revoked_at=now()`, `UPDATE pride.staff_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 second'`} {
		live := login(t, h, "owner")
		paused := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { exec(mutation); service.attendance(w, r) }))
		live.h = paused
		status(t, live.call("GET", reportPath, nil), 403)
		exec(`UPDATE pride.staff_users SET is_owner=true,disabled_at=NULL WHERE login='owner'`)
	}
}
