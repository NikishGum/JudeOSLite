package families

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
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
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
	name := fmt.Sprintf("rd09_%d", time.Now().UnixNano())
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
	f.handler = access.Handler(New(pool, access).Handler(corrections.New(pool, access).Handler(attendance.New(pool, access).Handler(participants.New(pool, access).Handler(clubs.New(pool, access).Handler(fallback))))))
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
func (f *fixture) family(c client, club, name string, ids ...string) Family {
	f.t.Helper()
	if ids == nil {
		ids = []string{}
	}
	w := f.call(c, "POST", "/api/clubs/"+club+"/families", map[string]any{"name": name, "contactName": "Private Contact", "phone": "555-private", "email": "private@example.test", "membershipIds": ids, "version": 0})
	f.status(w, 201)
	return familyJSON(f.t, w)
}
func familyJSON(t *testing.T, w *httptest.ResponseRecorder) Family {
	t.Helper()
	var f Family
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatal(err)
	}
	if f.Children == nil || f.Accounts == nil {
		t.Fatal("null arrays")
	}
	return f
}
func input(f Family, ids []string) map[string]any {
	return map[string]any{"name": f.Name, "contactName": f.ContactName, "phone": f.Phone, "email": f.Email, "membershipIds": ids, "version": f.Version}
}
func path(f Family) string               { return "/api/clubs/" + f.ClubID + "/families/" + f.ID }
func profilePath(club, id string) string { return "/api/clubs/" + club + "/member-profiles/" + id }
func (f *fixture) account(fam Family, login string) Family {
	f.t.Helper()
	w := f.call(f.owner, "POST", path(fam)+"/accounts", map[string]any{"login": login, "displayName": "Synthetic Parent", "password": password, "version": fam.Version})
	f.status(w, 201)
	return familyJSON(f.t, w)
}

func TestFamilyAccessLifecycle(t *testing.T) {
	f := setup(t)
	m1, m2, foreign := f.member(f.a, "Same Child"), f.member(f.a, "Same Child"), f.member(f.b, "Same Child")
	a := f.family(f.admin, f.a, "Private Family A", m1, m2)
	b := f.family(f.owner, f.b, "Private Family B", foreign)
	a = f.account(a, "parent-a")
	b = f.account(b, "parent-b")
	pa, pb := f.login("parent-a", password), f.login("parent-b", password)
	if len(a.Children) != 2 || a.Children[0].ID == a.Children[1].ID {
		t.Fatal("explicit children lost")
	}
	for _, c := range []client{pa, pb} {
		w := f.call(c, "GET", "/api/portal/families", nil)
		f.status(w, 200)
		var body struct {
			Items   []Family
			Total   int
			HasMore bool
		}
		json.Unmarshal(w.Body.Bytes(), &body)
		if body.Total != 1 || len(body.Items) != 1 || body.HasMore {
			t.Fatal(w.Body.String())
		}
		if c.csrf == pa.csrf && body.Items[0].ID != a.ID {
			t.Fatal("foreign family disclosed")
		}
	}
	me := f.call(pa, "GET", "/api/auth/me", nil)
	f.status(me, 200)
	if !strings.Contains(me.Body.String(), `"isParent":true`) || !strings.Contains(me.Body.String(), `"roles":["parent"]`) || strings.Contains(me.Body.String(), f.b) {
		t.Fatal(me.Body.String())
	}
	f.status(f.call(pa, "GET", "/api/auth/clubs/"+f.a, nil), 200)
	for _, p := range []string{profilePath(f.b, foreign), profilePath(f.a, foreign), profilePath(f.a, "not-uuid"), path(b), path(a), "/api/clubs/" + f.a + "/families", "/api/clubs/" + f.a + "/participants"} {
		f.status(f.call(pa, "GET", p, nil), 404)
	}
	for _, p := range []string{profilePath(f.a, m1), profilePath(f.a, m2)} {
		w := f.call(pa, "GET", p, nil)
		f.status(w, 200)
		var v MemberProfile
		json.Unmarshal(w.Body.Bytes(), &v)
		if v.Attendance.Rate != nil || v.FamilyID == nil || *v.FamilyID != a.ID || v.Groups == nil || v.Upcoming == nil || v.Attendance.History == nil {
			t.Fatal(w.Body.String())
		}
	}
	f.status(f.call(f.trainer, "GET", profilePath(f.a, m1), nil), 404)
	f.status(f.call(f.trainer, "GET", path(a), nil), 404)
	f.status(f.call(client{}, "GET", profilePath(f.a, m1), nil), 401)
	// CSRF and stale versions preserve both membership links and family version.
	bad := f.admin
	bad.csrf = ""
	f.status(f.call(bad, "PUT", path(a), input(a, []string{m1})), 403)
	stale := a
	stale.Version--
	f.status(f.call(f.admin, "PUT", path(a), input(stale, []string{m1})), 409)
	f.status(f.call(f.admin, "PUT", path(a), input(a, []string{foreign})), 404)
	f.status(f.call(f.admin, "PUT", path(a), input(a, []string{m1, m1})), 400)
	f.status(f.call(f.admin, "POST", path(a)+"/accounts", map[string]any{"login": "parent-b", "displayName": "X", "password": password, "version": a.Version}), 409)
	w := f.call(f.admin, "PUT", path(a), input(a, []string{m2}))
	f.status(w, 200)
	a = familyJSON(t, w)
	f.status(f.call(pa, "GET", profilePath(f.a, m1), nil), 404)
	f.status(f.call(pa, "GET", profilePath(f.a, m2), nil), 200)
	// Reset revokes every old session and preserves family access.
	user := a.Accounts[0].ID
	reset := path(a) + "/accounts/" + user + "/reset-password"
	w = f.call(f.admin, "POST", reset, map[string]any{"password": "new-synthetic-password", "version": a.Version})
	f.status(w, 200)
	a = familyJSON(t, w)
	f.status(f.call(pa, "GET", profilePath(f.a, m2), nil), 401)
	f.status(f.call(client{}, "POST", "/api/auth/login", map[string]any{"login": "parent-a", "password": password}), 401)
	pa = f.login("parent-a", "new-synthetic-password")
	// A staff+parent union must not let a local admin reset independent staff identity.
	f.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, user, f.a)
	w = f.call(pa, "GET", "/api/auth/me", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"parent","trainer"`) {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(f.admin, "POST", reset, map[string]any{"password": password, "version": a.Version}), 403)
	w = f.call(f.admin, "POST", path(a)+"/accounts/"+user+"/revoke", map[string]any{"version": a.Version})
	f.status(w, 200)
	a = familyJSON(t, w)
	f.status(f.call(pa, "GET", profilePath(f.a, m2), nil), 404)
	w = f.call(pa, "GET", "/api/auth/me", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"isParent":false`) || !strings.Contains(w.Body.String(), `"roles":["trainer"]`) {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(pa, "GET", "/api/clubs/"+f.a, nil), 200)
	// Contact/name/password material is absent from every family audit payload.
	audit := f.id(`SELECT coalesce(string_agg(after_state::text||coalesce(before_state::text,''),' '),'') FROM pride.audit_events WHERE entity_type='family'`)
	for _, secret := range []string{"Private", "555-private", "private@example", "password", "argon2", "Same Child", "Synthetic Parent", "parent-a"} {
		if strings.Contains(audit, secret) {
			t.Fatal("PII/credential audit disclosure")
		}
	}
	f.exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.a)
	f.status(f.call(f.admin, "PUT", path(a), input(a, []string{m2})), 409)
	f.status(f.call(f.admin, "GET", path(a), nil), 200)
	f.exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.adminID)
	f.status(f.call(f.admin, "GET", path(a), nil), 404)
}

func TestFamiliesPaginationConcurrencyAndArchive(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Concurrent Child")
	// Separate families race to claim the very same explicit membership.
	start := make(chan struct{})
	out := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"Race A", "Race B"} {
		wg.Go(func() {
			<-start
			out <- f.call(f.admin, "POST", "/api/clubs/"+f.a+"/families", map[string]any{"name": name, "contactName": "", "phone": "", "email": "", "membershipIds": []string{m}, "version": 0})
		})
	}
	close(start)
	wg.Wait()
	close(out)
	wins, conflicts := 0, 0
	var a Family
	for w := range out {
		switch w.Code {
		case 201:
			wins++
			a = familyJSON(t, w)
		case 409:
			conflicts++
		default:
			t.Fatal(w.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins %d conflicts %d", wins, conflicts)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.family_children WHERE membership_id=$1`, m); n != "1" {
		t.Fatal(n)
	}
	// Same version competing edits have one winner and one stale-write conflict.
	out = make(chan *httptest.ResponseRecorder, 2)
	start = make(chan struct{})
	for range 2 {
		wg.Go(func() { <-start; out <- f.call(f.admin, "PUT", path(a), input(a, []string{m})) })
	}
	close(start)
	wg.Wait()
	close(out)
	wins, conflicts = 0, 0
	for w := range out {
		if w.Code == 200 {
			wins++
			a = familyJSON(t, w)
		} else if w.Code == 409 {
			conflicts++
		} else {
			t.Fatal(w.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("lost family update")
	}
	// Pagination is bounded and stable across duplicate names.
	f.family(f.admin, f.a, "Page")
	f.family(f.admin, f.a, "Page")
	ids := map[string]bool{}
	for i := 0; i < 3; i++ {
		w := f.call(f.admin, "GET", fmt.Sprintf("/api/clubs/%s/families?page=%d&limit=1", f.a, i), nil)
		f.status(w, 200)
		var list struct {
			Items   []Family
			Total   int
			HasMore bool
		}
		json.Unmarshal(w.Body.Bytes(), &list)
		if len(list.Items) != 1 || list.Total != 3 || list.HasMore != (i < 2) || ids[list.Items[0].ID] {
			t.Fatal(w.Body.String())
		}
		ids[list.Items[0].ID] = true
	}
	for _, q := range []string{"limit=0", "limit=101", "page=-1", "page=x", "page=1000001"} {
		f.status(f.call(f.admin, "GET", "/api/clubs/"+f.a+"/families?"+q, nil), 400)
	}
	w := f.call(f.admin, "GET", "/api/clubs/"+f.a+"/families?q=absent", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal(w.Body.String())
	}
	a = f.account(a, "archive-parent")
	p := f.login("archive-parent", password)
	f.exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.a)
	f.status(f.call(p, "GET", profilePath(f.a, m), nil), 404)
	f.status(f.call(p, "GET", "/api/auth/clubs/"+f.a, nil), 404)
	w = f.call(p, "GET", "/api/portal/families", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal(w.Body.String())
	}
	w = f.call(p, "GET", "/api/auth/me", nil)
	f.status(w, 200)
	if strings.Contains(w.Body.String(), f.a) || !strings.Contains(w.Body.String(), `"isParent":false`) {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(f.admin, "POST", path(a)+"/accounts/"+a.Accounts[0].ID+"/revoke", map[string]any{"version": a.Version}), 200)
}

func TestMemberProfileAttendanceAndLiveReaderLocks(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Profile Child")
	fam := f.account(f.family(f.admin, f.a, "Profile Family", m), "profile-parent")
	p := f.login("profile-parent", password)
	g := f.id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Synthetic Group') RETURNING id::text`, f.a)
	source := f.id(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2020-01-01') RETURNING id::text`, f.a, g, m)
	sessions := []string{}
	// 25 closed records verify full totals while history is limited to 20.
	for i := 0; i < 25; i++ {
		session := f.id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,current_date-$4::int,'Europe/Minsk',(current_date-$4::int)+time '18:00' AT TIME ZONE 'Europe/Minsk',(current_date-$4::int)+time '19:00' AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, f.a, g, f.trainerID, i+1)
		sessions = append(sessions, session)
		f.exec(`INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5)`, f.a, session, g, m, source)
		f.exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=now() WHERE id=$1`, session)
		state, reason := "absent", any("illness")
		if i%2 == 0 {
			state, reason = "present", nil
		}
		f.exec(`UPDATE pride.attendance_records SET state=$2,absence_reason=$3,marked_by=$4,marked_at=now(),origin='manual' WHERE training_session_id=$1`, session, state, reason, f.ownerID)
		f.exec(`UPDATE pride.training_sessions SET status='closed',closed_at=now() WHERE id=$1`, session)
	}
	// A cancelled snapshot is never counted; an open record remains null in history.
	for _, state := range []string{"cancelled", "open"} {
		session := f.id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,current_date,'Europe/Minsk',now()+$4::interval,now()+$4::interval+interval '1 hour') RETURNING id::text`, f.a, g, f.trainerID, map[string]string{"cancelled": "-4 hours", "open": "-2 hours"}[state])
		f.exec(`INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5)`, f.a, session, g, m, source)
		f.exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=now() WHERE id=$1`, session)
		if state == "cancelled" {
			f.exec(`UPDATE pride.training_sessions SET status='cancelled',cancelled_at=now(),cancellation_reason='Synthetic' WHERE id=$1`, session)
		}
	}
	for i := 0; i < 23; i++ {
		f.id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,current_date+$4::int,'Europe/Minsk',(current_date+$4::int)+time '18:00' AT TIME ZONE 'Europe/Minsk',(current_date+$4::int)+time '19:00' AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, f.a, g, f.trainerID, i+1)
	}
	w := f.call(p, "GET", profilePath(f.a, m), nil)
	f.status(w, 200)
	var v MemberProfile
	if json.Unmarshal(w.Body.Bytes(), &v) != nil {
		t.Fatal("bad profile")
	}
	if v.Attendance.Present != 13 || v.Attendance.Absent != 12 || v.Attendance.Rate == nil || *v.Attendance.Rate != 52 || len(v.Attendance.History) != 20 || len(v.Upcoming) != 20 || len(v.Groups) != 1 || v.GroupName != "Synthetic Group" || v.TrainerName != "Trainer" {
		t.Fatal(w.Body.String())
	}
	if v.Attendance.History[0].State != nil || v.Groups[0].Until != nil {
		t.Fatal("missing nullable fields")
	}
	// A genuine correction is reflected by the next profile read, not stale counters.
	f.exec(`UPDATE pride.attendance_records SET state='absent',absence_reason='excused',origin='correction',change_reason='Synthetic correction',version=version+1 WHERE training_session_id=$1`, sessions[0])
	w = f.call(p, "GET", profilePath(f.a, m), nil)
	f.status(w, 200)
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.Attendance.Present != 12 || v.Attendance.Absent != 13 || *v.Attendance.Rate != 48 {
		t.Fatal(w.Body.String())
	}
	// The same identity can have a second local card; family A must not reach it.
	person := f.id(`SELECT participant_id::text FROM pride.club_memberships WHERE id=$1`, m)
	foreign := f.id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2020-01-01','phone','private') RETURNING id::text`, f.b, person)
	f.status(f.call(p, "GET", profilePath(f.b, foreign), nil), 404)
	// Prove the reusable helper holds live relationship locks until data is read.
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// Invoke through Require to use a genuine authenticated principal.
	access, err := auth.New(f.pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	handler := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := LockMemberReader(r.Context(), tx, f.a, m); err != nil {
			t.Error(err)
		}
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", profilePath(f.a, m), nil)
	for _, c := range p.cookies {
		req.AddCookie(c)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = f.pool.Exec(ctx, `DELETE FROM pride.family_children WHERE family_id=$1 AND membership_id=$2`, fam.ID, m)
	cancel()
	if err == nil {
		t.Fatal("relationship revoked inside protected read")
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.exec(`DELETE FROM pride.family_children WHERE family_id=$1 AND membership_id=$2`, fam.ID, m)
	f.status(f.call(p, "GET", profilePath(f.a, m), nil), 404)
}

func TestSharedStaffIdentityCannotBeChangedThroughTrainerAPI(t *testing.T) {
	f := setup(t)
	m := f.member(f.b, "Private Child")
	fam := f.family(f.owner, f.b, "Private Family", m)
	f.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, fam.ID, f.trainerID)
	// Previously this account had only one trainer assignment in A. Its new
	// independent parent rights in B make the global identity owner-managed.
	version := f.id(`SELECT version::text FROM pride.staff_users WHERE id=$1`, f.trainerID)
	f.status(f.call(f.admin, "POST", "/api/clubs/"+f.a+"/trainers/"+f.trainerID+"/password", map[string]any{"password": password, "version": json.Number(version)}), 403)
	f.status(f.call(f.admin, "PUT", "/api/clubs/"+f.a+"/trainers/"+f.trainerID+"/account", map[string]any{"login": "trainer", "displayName": "Changed", "disabled": true, "version": json.Number(version)}), 403)
	w := f.call(f.admin, "GET", "/api/clubs/"+f.a+"/trainers", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"canManageAccount":false`) {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(f.trainer, "GET", profilePath(f.b, m), nil), 200)
	// Owner may reset a shared parent identity; all staff roles survive revoke.
	w = f.call(f.owner, "POST", path(fam)+"/accounts/"+f.trainerID+"/reset-password", map[string]any{"password": password, "version": fam.Version})
	f.status(w, 200)
	fam = familyJSON(t, w)
	f.status(f.call(f.trainer, "GET", profilePath(f.b, m), nil), 401)
	f.trainer = f.login("trainer", password)
	f.status(f.call(f.owner, "POST", path(fam)+"/accounts/"+f.trainerID+"/revoke", map[string]any{"version": fam.Version}), 200)
	f.status(f.call(f.trainer, "GET", profilePath(f.b, m), nil), 404)
	f.status(f.call(f.trainer, "GET", "/api/auth/clubs/"+f.a, nil), 200)
}

func TestSameClubIsolationAndMalformedWrites(t *testing.T) {
	f := setup(t)
	own, other := f.member(f.a, "Same Name"), f.member(f.a, "Same Name")
	a := f.account(f.family(f.admin, f.a, "Same Family Name", own), "local-parent")
	b := f.family(f.admin, f.a, "Same Family Name", other)
	parent := f.login("local-parent", password)
	f.status(f.call(parent, "GET", profilePath(f.a, other), nil), 404)
	f.status(f.call(parent, "GET", path(b), nil), 404)
	f.status(f.call(parent, "GET", profilePath(f.a, own), nil), 200)
	listed := f.call(parent, "GET", "/api/portal/families?q=Same&limit=1", nil)
	f.status(listed, 200)
	var page struct {
		Items []Family
		Total int
	}
	json.Unmarshal(listed.Body.Bytes(), &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != a.ID {
		t.Fatal(listed.Body.String())
	}
	for _, body := range []map[string]any{
		{"name": "X", "membershipIds": []string{}},
		{"name": "X", "membershipIds": []string{}, "version": nil},
		{"name": "X", "membershipIds": nil, "version": 0},
		{"name": "X", "membershipIds": []string{"invalid"}, "version": 0},
		{"name": "X", "membershipIds": []string{}, "version": 1},
		{"name": "X\nY", "membershipIds": []string{}, "version": 0},
		{"name": "X", "membershipIds": []string{}, "version": 0, "unexpected": "value"},
	} {
		body["contactName"] = ""
		body["phone"] = ""
		body["email"] = ""
		f.status(f.call(f.admin, "POST", "/api/clubs/"+f.a+"/families", body), 400)
	}
	for _, raw := range []string{`null`, `{"name":"X"} {}`, strings.Repeat("x", 33000)} {
		req := httptest.NewRequest("POST", "/api/clubs/"+f.a+"/families", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		req.Header.Set("X-CSRF-Token", f.admin.csrf)
		for _, c := range f.admin.cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, req)
		f.status(w, 400)
	}
	// Creating a parent role never grants family/account mutations to that parent.
	f.status(f.call(parent, "POST", path(a)+"/accounts", map[string]any{"login": "new", "displayName": "New", "password": password, "version": a.Version}), 404)
	for _, pass := range []string{"short", strings.Repeat("x", 129), strings.Repeat("😀", 40)} {
		f.status(f.call(f.admin, "POST", path(a)+"/accounts", map[string]any{"login": "new", "displayName": "New", "password": pass, "version": a.Version}), 400)
	}
	f.exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, a.Accounts[0].ID)
	f.status(f.call(parent, "GET", profilePath(f.a, own), nil), 401)
	// Staff-only metadata remains intact for an owner with parent access as well.
	f.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, a.ID, f.ownerID)
	me := f.call(f.owner, "GET", "/api/auth/me", nil)
	f.status(me, 200)
	if !strings.Contains(me.Body.String(), `"owner","parent"`) || !strings.Contains(me.Body.String(), `"isParent":true`) {
		t.Fatal(me.Body.String())
	}
}
