package attendance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type fixture struct {
	pool                                                               *pgxpool.Pool
	service                                                            *Service
	trainer, replacement, owner                                        client
	club, otherClub, group, emptyGroup, trainerID, replacementID, base string
}

func setup(t *testing.T) *fixture {
	pool := database(t)
	ctx := context.Background()
	if e := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); e != nil {
		t.Fatal(e)
	}
	access, e := auth.New(pool, auth.Config{Origin: origin})
	if e != nil {
		t.Fatal(e)
	}
	s := New(pool, access)
	s.now = func() time.Time { return time.Date(2028, 1, 2, 21, 30, 0, 0, time.UTC) } // Jan 3 in Minsk.
	h := access.Handler(s.Handler(sessions.New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler()))))
	f := &fixture{pool: pool, service: s, owner: login(t, h, "owner")}
	f.club = parse[clubs.Club](t, f.owner.call("POST", "/api/clubs", map[string]string{"name": "A", "timezone": "Europe/Minsk"}), 201).ID
	f.otherClub = parse[clubs.Club](t, f.owner.call("POST", "/api/clubs", map[string]string{"name": "PRIVATE B", "timezone": "UTC"}), 201).ID
	for _, g := range []*string{&f.group, &f.emptyGroup} {
		*g = f.id(t, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Synthetic group') RETURNING id::text`, f.club)
	}
	f.trainerID = f.staff(t, "trainer", f.club)
	f.replacementID = f.staff(t, "replacement", f.club)
	f.trainer = login(t, h, "trainer")
	f.replacement = login(t, h, "replacement")
	f.base = "/api/clubs/" + f.club + "/trainer-sessions"
	return f
}
func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, e := f.pool.Exec(context.Background(), q, args...); e != nil {
		t.Fatal(e)
	}
}
func (f *fixture) id(t *testing.T, q string, args ...any) string {
	t.Helper()
	var id string
	if e := f.pool.QueryRow(context.Background(), q, args...).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func (f *fixture) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if e := f.pool.QueryRow(context.Background(), q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func (f *fixture) staff(t *testing.T, name, club string) string {
	id := f.id(t, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name)
	f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, id, club)
	return id
}
func (f *fixture) member(t *testing.T, name, from string, until *string) string {
	p := f.id(t, `INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'2015-01-01') RETURNING id::text`, name)
	c := f.id(t, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2020-01-01','phone','synthetic-private-contact') RETURNING id::text`, p, f.club)
	f.exec(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from,valid_until) VALUES($1,$2,$3,$4::date,$5::date)`, f.club, f.group, c, from, until)
	return c
}
func (f *fixture) newSession(t *testing.T, group, date string) sessions.Session {
	id := f.id(t, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4::date,'Europe/Minsk',($4::date+TIME '18:00') AT TIME ZONE 'Europe/Minsk',($4::date+TIME '19:00') AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, f.club, group, f.trainerID, date)
	return parse[Journal](t, f.trainer.call("GET", f.path(id), nil), 200).Session
}
func (f *fixture) path(id string) string { return f.base + "/" + id + "/journal" }
func (f *fixture) open(t *testing.T, v sessions.Session) Journal {
	return parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/open", versionInput{v.Version}), 200)
}
func (f *fixture) mark(t *testing.T, j Journal, index int, state string, reason *string) Journal {
	return parse[Journal](t, f.trainer.call("PUT", f.path(j.Session.ID)+"/records/"+j.Records[index].ID, markInput{j.Records[index].Version, j.Session.Version, state, reason}), 200)
}
func ptr(s string) *string { return &s }
func TestJournalIntegration(t *testing.T) {
	f := setup(t)
	old := f.member(t, "Gone on boundary", "2027-01-01", ptr("2028-01-03"))
	a := f.member(t, "A Synthetic", "2028-01-03", nil)
	b := f.member(t, "B Synthetic", "2027-01-01", nil)
	f.member(t, "Future", "2028-01-04", nil)
	v := f.newSession(t, f.group, "2028-01-03")
	list := parse[[]sessions.Session](t, f.trainer.call("GET", f.base, nil), 200)
	if len(list) != 1 || list[0].ID != v.ID {
		t.Fatal("today must use local timezone")
	}
	status(t, f.trainer.call("GET", f.base+"?date=2028-02-30", nil), 400)
	status(t, f.owner.call("GET", f.base, nil), 404)
	status(t, f.replacement.call("GET", f.path(v.ID), nil), 404)
	status(t, f.trainer.call("GET", "/api/clubs/"+f.otherClub+"/trainer-sessions/"+v.ID+"/journal", nil), 404)
	status(t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{1}), 409)
	status(t, f.trainer.call("POST", f.path(v.ID)+"/open", map[string]any{"version": 1, "trainerId": f.trainerID}), 400)
	noCSRF := f.trainer
	noCSRF.csrf = ""
	status(t, noCSRF.call("POST", f.path(v.ID)+"/open", versionInput{1}), 403)
	// An otherwise valid snapshot source cannot be attached to another group.
	planned := f.newSession(t, f.emptyGroup, "2028-01-01")
	if _, e := f.pool.Exec(context.Background(), `INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) SELECT $1,$2,$3,m.club_membership_id,m.id FROM pride.group_memberships m WHERE m.club_membership_id=$4`, f.club, planned.ID, f.emptyGroup, b); e == nil {
		t.Fatal("cross-group snapshot FK accepted")
	}
	j := f.open(t, v)
	if len(j.Records) != 2 || j.Records[0].MembershipID != a || j.Records[1].MembershipID != b || !j.Session.Frozen {
		t.Fatalf("snapshot: %+v", j)
	}
	// Updating dated composition and archive after opening never rebuilds snapshot.
	f.exec(t, `UPDATE pride.group_memberships SET valid_until='2028-01-04' WHERE club_membership_id=$1`, a)
	f.exec(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2028-01-04')`, f.club, f.emptyGroup, a)
	f.exec(t, `UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, b)
	f.exec(t, `UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, f.group)
	for range 2 {
		again := f.open(t, v)
		if len(again.Records) != 2 || again.Records[0].ID != j.Records[0].ID || !again.Records[1].Archived || again.Session.Version != j.Session.Version {
			t.Fatal("snapshot rebuilt")
		}
	}
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE club_membership_id=$1`, old) != 0 {
		t.Fatal("exclusive boundary")
	}
	// Same command is safe; changed stale command fails.
	cmd := markInput{j.Records[0].Version, j.Session.Version, "present", nil}
	j = f.mark(t, j, 0, "present", nil)
	same := parse[Journal](t, f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[0].ID, cmd), 200)
	if same.Session.Version != j.Session.Version || !same.Records[0].MarkedAt.Equal(*j.Records[0].MarkedAt) {
		t.Fatal("idempotent mark changed history")
	}
	cmd.State = "absent"
	cmd.Reason = ptr("illness")
	status(t, f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[0].ID, cmd), 409)
	for _, reason := range []string{"unknown", "illness", "excused"} {
		j = f.mark(t, j, 0, "absent", ptr(reason))
	}
	j = f.mark(t, j, 0, "present", nil)
	status(t, f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[1].ID, markInput{1, j.Session.Version, "present", ptr("illness")}), 400)
	status(t, f.trainer.call("PUT", f.path(v.ID)+"/records/not-a-record", markInput{1, j.Session.Version, "present", nil}), 404)
	status(t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{2}), 409)
	version := j.Session.Version
	j = parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{version}), 200)
	if j.Session.Status != "closed" || j.Session.ClosedAt == nil || *j.Records[1].State != "absent" || *j.Records[1].Reason != "unknown" || *j.Records[1].Origin != "close_default" || *j.Records[0].Origin != "manual" {
		t.Fatal("close not complete")
	}
	auditCount := f.count(t, `SELECT count(*) FROM pride.audit_events`)
	parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{version}), 200)
	parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/open", versionInput{1}), 200)
	if f.count(t, `SELECT count(*) FROM pride.audit_events`) != auditCount {
		t.Fatal("repeat mutated history")
	}
	status(t, f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[0].ID, markInput{j.Records[0].Version, j.Session.Version, "present", nil}), 409)
	if f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record' AND actor_id=$1 AND after_state->>'origin'='close_default' AND before_state->'state'='null'::jsonb`, f.trainerID) != 1 {
		t.Fatal("missing automatic absence provenance")
	}
	if f.count(t, `SELECT count(*) FROM pride.audit_events WHERE before_state::text LIKE '%Synthetic%' OR after_state::text LIKE '%synthetic-private-contact%'`) != 0 {
		t.Fatal("audit leaks personal fields")
	}
	empty := f.open(t, f.newSession(t, f.emptyGroup, "2028-01-02"))
	if len(empty.Records) != 0 {
		t.Fatal("empty snapshot")
	}
	empty = parse[Journal](t, f.trainer.call("POST", f.path(empty.Session.ID)+"/close", versionInput{empty.Session.Version}), 200)
	if empty.Session.Status != "closed" {
		t.Fatal("empty close")
	}
	future := f.newSession(t, f.emptyGroup, "2028-01-04")
	status(t, f.trainer.call("POST", f.path(future.ID)+"/open", versionInput{1}), 409)
	// Relocated occurrence uses actual date rather than source starts_on.
	moved := f.newSession(t, f.emptyGroup, "2027-12-30")
	f.exec(t, `UPDATE pride.training_sessions SET starts_at='2028-01-03 18:00+03',ends_at='2028-01-03 19:00+03',is_manual_override=true WHERE id=$1`, moved.ID)
	movedJ := f.open(t, moved)
	if movedJ.Session.Date != "2028-01-03" || movedJ.Session.SourceDate != "2027-12-30" {
		t.Fatal("wrong actual date")
	}
	// DB integrity protects keys/terminal records even without HTTP.
	for _, q := range []string{`DELETE FROM pride.attendance_records WHERE id=$1`, `UPDATE pride.attendance_records SET state=NULL,absence_reason=NULL,marked_by=NULL,marked_at=NULL,origin=NULL WHERE id=$1`, `UPDATE pride.attendance_records SET club_id='00000000-0000-0000-0000-000000000000' WHERE id=$1`} {
		if _, e := f.pool.Exec(context.Background(), q, j.Records[0].ID); e == nil {
			t.Fatal("direct SQL corrupted snapshot")
		}
	}
}
func TestJournalRollback(t *testing.T) {
	f := setup(t)
	f.member(t, "A", "2027-01-01", nil)
	f.member(t, "B", "2027-01-01", nil)
	v := f.newSession(t, f.group, "2028-01-03")
	reject := func(action string) {
		f.exec(t, `CREATE OR REPLACE FUNCTION pride.reject_journal_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='`+action+`' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END; $$`)
		f.exec(t, `CREATE TRIGGER reject_journal_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_journal_audit()`)
	}
	drop := func() { f.exec(t, `DROP TRIGGER reject_journal_audit ON pride.audit_events`) }
	reject("journal_opened")
	status(t, f.trainer.call("POST", f.path(v.ID)+"/open", versionInput{1}), 503)
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1`, v.ID) != 0 || f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND status='planned' AND roster_frozen_at IS NULL AND version=1`, v.ID) != 1 {
		t.Fatal("partial open")
	}
	drop()
	j := f.open(t, v)
	reject("marked")
	status(t, f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[0].ID, markInput{1, j.Session.Version, "present", nil}), 503)
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1 AND state IS NOT NULL`, v.ID) != 0 {
		t.Fatal("partial mark")
	}
	drop()
	// Fail after all auto marks and their audits: the entire close must roll back.
	reject("journal_closed")
	status(t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{j.Session.Version}), 503)
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1 AND state IS NOT NULL`, v.ID) != 0 || f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_type='attendance_record'`) != 0 || f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND status='open' AND closed_at IS NULL AND version=$2`, v.ID, j.Session.Version) != 1 {
		t.Fatal("partial close")
	}
	drop()
	parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{j.Session.Version}), 200)
}
func TestJournalConcurrency(t *testing.T) {
	f := setup(t)
	f.member(t, "A", "2027-01-01", nil)
	for i := 0; i < 12; i++ {
		v := f.newSession(t, f.group, fmt.Sprintf("2027-12-%02d", i+1))
		start := make(chan struct{})
		out := make(chan *httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() { <-start; out <- f.trainer.call("POST", f.path(v.ID)+"/open", versionInput{1}) })
		}
		close(start)
		wg.Wait()
		close(out)
		for w := range out {
			status(t, w, 200)
		}
		j := parse[Journal](t, f.trainer.call("GET", f.path(v.ID), nil), 200)
		if len(j.Records) != 1 {
			t.Fatal("double snapshot")
		}
		start = make(chan struct{})
		out = make(chan *httptest.ResponseRecorder, 2)
		wg.Go(func() {
			<-start
			out <- f.trainer.call("PUT", f.path(v.ID)+"/records/"+j.Records[0].ID, markInput{1, j.Session.Version, "present", nil})
		})
		wg.Go(func() { <-start; out <- f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{j.Session.Version}) })
		close(start)
		wg.Wait()
		close(out)
		successes := 0
		for w := range out {
			if w.Code == 200 {
				successes++
			} else {
				status(t, w, 409)
			}
		}
		if successes != 1 {
			t.Fatal("race did not serialize")
		}
		j = parse[Journal](t, f.trainer.call("GET", f.path(v.ID), nil), 200)
		if j.Session.Status == "open" {
			j = parse[Journal](t, f.trainer.call("POST", f.path(v.ID)+"/close", versionInput{j.Session.Version}), 200)
		}
		if j.Session.Status != "closed" || j.Records[0].State == nil {
			t.Fatal("ambiguous mark/close state")
		}
	}
	for i := 0; i < 8; i++ {
		j := f.open(t, f.newSession(t, f.group, fmt.Sprintf("2027-11-%02d", i+1)))
		start := make(chan struct{})
		out := make(chan *httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		wg.Go(func() {
			<-start
			out <- f.trainer.call("POST", f.path(j.Session.ID)+"/close", versionInput{j.Session.Version})
		})
		wg.Go(func() {
			<-start
			out <- f.owner.call("POST", "/api/clubs/"+f.club+"/sessions/"+j.Session.ID+"/cancel", map[string]any{"version": j.Session.Version, "reason": "Synthetic cancel", "confirmOpen": true})
		})
		close(start)
		wg.Wait()
		close(out)
		successes := 0
		for w := range out {
			if w.Code == 200 {
				successes++
			} else {
				status(t, w, 409)
			}
		}
		if successes != 1 {
			t.Fatal("cancel/close race")
		}
		current := parse[Journal](t, f.trainer.call("GET", f.path(j.Session.ID), nil), 200)
		if current.Session.Status == "cancelled" && current.Records[0].State != nil {
			t.Fatal("cancel created absence")
		}
		if current.Session.Status == "closed" && current.Records[0].State == nil {
			t.Fatal("close incomplete")
		}
	}
}
func TestJournalReplacementAndRevocation(t *testing.T) {
	f := setup(t)
	f.member(t, "A", "2027-01-01", nil)
	j := f.open(t, f.newSession(t, f.group, "2028-01-03"))
	j = f.mark(t, j, 0, "absent", ptr("illness"))
	parse[sessions.Session](t, f.owner.call("POST", "/api/clubs/"+f.club+"/sessions/"+j.Session.ID+"/replace-trainer", map[string]any{"version": j.Session.Version, "trainerId": f.replacementID}), 200)
	status(t, f.trainer.call("GET", f.path(j.Session.ID), nil), 404)
	status(t, f.trainer.call("POST", f.path(j.Session.ID)+"/close", versionInput{j.Session.Version}), 404)
	next := parse[Journal](t, f.replacement.call("GET", f.path(j.Session.ID), nil), 200)
	if *next.Records[0].MarkedBy != f.trainerID {
		t.Fatal("replacement rewrote author")
	}
	// Current role and session are rechecked after Require middleware.
	domain := http.NewServeMux()
	var after func()
	access, e := auth.New(f.pool, auth.Config{Origin: origin})
	if e != nil {
		t.Fatal(e)
	}
	hook := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		after()
		if r.Method == "GET" {
			f.service.get(w, r)
		} else {
			f.service.close(w, r)
		}
	}))
	domain.Handle("GET /api/clubs/{clubID}/trainer-sessions/{sessionID}/journal", hook)
	domain.Handle("POST /api/clubs/{clubID}/trainer-sessions/{sessionID}/journal/close", hook)
	liveHandler := domain
	changed := f.replacement
	changed.h = liveHandler
	after = func() { f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.replacementID) }
	status(t, changed.call("GET", f.path(j.Session.ID), nil), 404)
	f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, f.replacementID, f.club)
	f.replacement = login(t, f.replacement.h, "replacement")
	changed = f.replacement
	changed.h = liveHandler
	after = func() {
		f.exec(t, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1`, f.replacementID)
	}
	status(t, changed.call("POST", f.path(j.Session.ID)+"/close", versionInput{next.Session.Version}), 404)
	// Account disable between middleware and transaction also forbids writes.
	f.replacement = login(t, f.replacement.h, "replacement")
	changed = f.replacement
	changed.h = liveHandler
	after = func() { f.exec(t, `UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, f.replacementID) }
	status(t, changed.call("POST", f.path(j.Session.ID)+"/close", versionInput{next.Session.Version}), 404)
	f.exec(t, `UPDATE pride.staff_users SET disabled_at=NULL WHERE id=$1`, f.replacementID)
	// Archive only forbids mutations; snapshot is retained/readable.
	f.replacement = login(t, f.replacement.h, "replacement")
	f.exec(t, `UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.club)
	parse[Journal](t, f.replacement.call("GET", f.path(j.Session.ID), nil), 200)
	status(t, f.replacement.call("POST", f.path(j.Session.ID)+"/close", versionInput{next.Session.Version}), 409)
}
