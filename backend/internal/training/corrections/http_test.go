package corrections

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
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
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
	service                                                            *attendance.Service
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
	s := attendance.New(pool, access)
	h := access.Handler(New(pool, access).Handler(s.Handler(sessions.New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler())))))
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
	c := f.id(t, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'1990-01-01','phone','synthetic-private-contact') RETURNING id::text`, p, f.club)
	f.exec(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from,valid_until) VALUES($1,$2,$3,$4::date,$5::date)`, f.club, f.group, c, from, until)
	return c
}
func (f *fixture) newSession(t *testing.T, group, date string) sessions.Session {
	id := f.id(t, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4::date,'Europe/Minsk',($4::date+TIME '18:00') AT TIME ZONE 'Europe/Minsk',($4::date+TIME '19:00') AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, f.club, group, f.trainerID, date)
	return parse[attendance.Journal](t, f.trainer.call("GET", f.path(id), nil), 200).Session
}
func (f *fixture) path(id string) string { return f.base + "/" + id + "/journal" }
func (f *fixture) open(t *testing.T, v sessions.Session) attendance.Journal {
	return parse[attendance.Journal](t, f.trainer.call("POST", f.path(v.ID)+"/open", map[string]any{"version": v.Version}), 200)
}
func (f *fixture) mark(t *testing.T, j attendance.Journal, index int, state string, reason *string) attendance.Journal {
	return parse[attendance.Journal](t, f.trainer.call("PUT", f.path(j.Session.ID)+"/records/"+j.Records[index].ID, map[string]any{"version": j.Records[index].Version, "sessionVersion": j.Session.Version, "state": state, "absenceReason": reason}), 200)
}
func ptr(s string) *string                    { return &s }
func (f *fixture) adminPath(id string) string { return "/api/clubs/" + f.club + "/journals/" + id }
func (f *fixture) closed(t *testing.T, date string) Journal {
	t.Helper()
	v := f.newSession(t, f.group, date)
	j := f.open(t, v)
	status(t, f.trainer.call("POST", f.path(v.ID)+"/close", map[string]any{"version": j.Session.Version}), 200)
	return parse[Journal](t, f.owner.call("GET", f.adminPath(v.ID), nil), 200)
}
func (f *fixture) administrator(t *testing.T, name, club string) (client, string) {
	id := f.staff(t, name, club)
	f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, id, club)
	return login(t, f.owner.h, name), id
}
func TestCorrectionsIntegration(t *testing.T) {
	f := setup(t)
	f.member(t, "Archived synthetic", "2000-01-01", nil)
	f.member(t, "Other synthetic", "2000-01-01", nil)
	admin, adminID := f.administrator(t, "administrator", f.club)
	other, _ := f.administrator(t, "foreign-admin", f.otherClub)
	j := f.closed(t, "2000-02-01")
	path := f.adminPath(j.Session.ID)
	record := j.Records[0]
	target := path + "/records/" + record.ID
	f.exec(t, `UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, record.MembershipID)
	f.exec(t, `UPDATE pride.group_memberships SET valid_until='2000-02-02' WHERE club_membership_id=$1`, record.MembershipID)
	f.exec(t, `UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, f.group)
	list := parse[[]sessions.Session](t, admin.call("GET", "/api/clubs/"+f.club+"/journals?date=2000-02-01&groupId="+f.group, nil), 200)
	if len(list) != 1 || list[0].ID != j.Session.ID {
		t.Fatal("historical journal missing")
	}
	for _, c := range []client{f.trainer, other} {
		status(t, c.call("GET", path, nil), 404)
		status(t, c.call("GET", target+"/history", nil), 404)
		status(t, c.call("PUT", target, Input{record.Version, "present", nil, "valid reason"}), 404)
	}
	status(t, admin.call("GET", "/api/clubs/"+f.club+"/journals?date=2000-02-30", nil), 400)
	status(t, admin.call("GET", "/api/clubs/"+f.club+"/journals?date=2000-02-01&groupId=invalid", nil), 404)
	status(t, admin.call("GET", "/api/clubs/"+f.otherClub+"/journals/"+j.Session.ID, nil), 404)
	for _, reason := range []string{"", "   ", "\n\t", "bad\x00reason", strings.Repeat("я", 1001)} {
		status(t, admin.call("PUT", target, Input{record.Version, "present", nil, reason}), 400)
	}
	for _, in := range []Input{{0, "present", nil, "reason"}, {record.Version, "present", ptr("illness"), "reason"}, {record.Version, "absent", nil, "reason"}, {record.Version, "absent", ptr("bad"), "reason"}, {record.Version, "other", nil, "reason"}} {
		status(t, admin.call("PUT", target, in), 400)
	}
	status(t, admin.call("PUT", target, map[string]any{"version": record.Version, "state": "present", "changeReason": "reason", "records": []string{record.ID}}), 400)
	noCSRF := admin
	noCSRF.csrf = ""
	status(t, noCSRF.call("PUT", target, Input{record.Version, "present", nil, "reason"}), 403)
	status(t, admin.call("PUT", path+"/records/"+j.Records[1].ID+"x", Input{record.Version, "present", nil, "reason"}), 404)
	status(t, f.owner.call("GET", "/api/clubs/invalid/journals?date=2000-02-01", nil), 404)
	foreignGroup := f.id(t, `INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Foreign') RETURNING id::text`, f.otherClub)
	status(t, admin.call("GET", "/api/clubs/"+f.club+"/journals?date=2000-02-01&groupId="+foreignGroup, nil), 404)
	beforeSession := j.Session
	updated := parse[Journal](t, admin.call("PUT", target, Input{record.Version, "present", nil, "  Исправлена ошибка тренера  "}), 200)
	if updated.Roster.Source != "snapshot" || updated.Roster.Summary == nil || updated.Roster.Summary.Present != 1 || updated.Roster.Summary.Absent != 1 || len(updated.Roster.Members) != 2 {
		t.Fatalf("correction summary disagrees with records: %+v", updated.Roster)
	}
	got := updated.Records[0]
	if got.State == nil || *got.State != "present" || got.Reason != nil || *got.Origin != "correction" || *got.MarkedBy != adminID || *got.ChangeReason != "Исправлена ошибка тренера" || got.Version != record.Version+1 || !got.Archived || got.MembershipID != record.MembershipID || got.SourceID != record.SourceID {
		t.Fatalf("invalid corrected historical record %+v", got)
	}
	if updated.Session.Version != beforeSession.Version || updated.Session.Status != "closed" || !updated.Session.ClosedAt.Equal(*beforeSession.ClosedAt) || len(updated.Records) != 2 || updated.Records[1].Version != j.Records[1].Version {
		t.Fatal("session, snapshot or unrelated mark changed")
	}
	history := parse[[]Event](t, admin.call("GET", target+"/history", nil), 200)
	if len(history) != 2 || history[1].Action != "corrected" || *history[1].Before.State != "absent" || *history[1].Before.Reason != "unknown" || *history[1].After.State != "present" || history[1].After.Reason != nil || history[1].ActorID != adminID || *history[1].ChangeReason != *got.ChangeReason || history[1].Before.MarkedBy == nil || history[1].Before.MarkedAt == nil {
		t.Fatalf("incomplete history %+v", history)
	}
	raw := admin.call("GET", target+"/history", nil).Body.String()
	if strings.Contains(raw, "synthetic-private-contact") || strings.Contains(raw, "password") || strings.Contains(raw, "fullName") || strings.Contains(raw, "birth") {
		t.Fatal("private payload leaked")
	}
	// Current totals already consume the corrected value, not audit history.
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1 AND state='present'`, j.Session.ID) != 1 {
		t.Fatal("present totals not corrected")
	}
	// A change to another record must not invalidate this record's version.
	parse[Journal](t, admin.call("PUT", path+"/records/"+j.Records[1].ID, Input{j.Records[1].Version, "absent", ptr("illness"), "other record"}), 200)
	// Identical current command is a no-op; stale commands always conflict.
	parse[Journal](t, admin.call("PUT", target, Input{got.Version, "present", nil, "another reason"}), 200)
	if f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1`, record.ID) != 2 {
		t.Fatal("no-op created history")
	}
	status(t, admin.call("PUT", target, Input{record.Version, "present", nil, "stale same"}), 409)
	for _, reason := range []string{"illness", "excused", "unknown"} {
		next := parse[Journal](t, admin.call("PUT", target, Input{got.Version, "absent", ptr(reason), "Отсутствовал: " + reason}), 200)
		got = next.Records[0]
	}
	status(t, f.trainer.call("PUT", f.path(j.Session.ID)+"/records/"+record.ID, map[string]any{"version": got.Version, "sessionVersion": j.Session.Version, "state": "present", "absenceReason": nil}), 409)
	trainerJournal := parse[attendance.Journal](t, f.trainer.call("GET", f.path(j.Session.ID), nil), 200)
	if *trainerJournal.Records[0].State != "absent" || *trainerJournal.Records[0].Reason != "unknown" {
		t.Fatal("trainer cannot read corrected values")
	}
	if strings.Contains(f.trainer.call("GET", f.path(j.Session.ID), nil).Body.String(), "Отсутствовал:") {
		t.Fatal("correction text leaked to trainer")
	}
	// Independent SQL totals consume current records, old values remain only in audit.
	if f.count(t, `SELECT count(*) FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE s.id=$1 AND s.status='closed' AND a.state='absent'`, j.Session.ID) != 2 {
		t.Fatal("current totals stale")
	}
	f.exec(t, `UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.club)
	status(t, admin.call("GET", path, nil), 200)
	status(t, admin.call("PUT", target, Input{got.Version, "present", nil, "reason"}), 409)
}
func TestCorrectionConcurrencyAndRollback(t *testing.T) {
	f := setup(t)
	f.member(t, "Synthetic", "2000-01-01", nil)
	admin, _ := f.administrator(t, "admin", f.club)
	for i := 0; i < 8; i++ {
		j := f.closed(t, fmt.Sprintf("2001-01-%02d", i+1))
		path := f.adminPath(j.Session.ID) + "/records/" + j.Records[0].ID
		codes := make(chan int, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, in := range []Input{{j.Records[0].Version, "present", nil, "concurrent A"}, {j.Records[0].Version, "absent", ptr("illness"), "concurrent B"}} {
			wg.Go(func() { <-start; codes <- admin.call("PUT", path, in).Code })
		}
		close(start)
		wg.Wait()
		close(codes)
		success, conflict := 0, 0
		for code := range codes {
			if code == 200 {
				success++
			} else if code == 409 {
				conflict++
			} else {
				t.Fatalf("unexpected race status %d", code)
			}
		}
		if success != 1 || conflict != 1 || f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1 AND action='corrected'`, j.Records[0].ID) != 1 {
			t.Fatal("correction overwritten")
		}
	}
	j := f.closed(t, "2002-01-01")
	path := f.adminPath(j.Session.ID) + "/records/" + j.Records[0].ID
	f.exec(t, `CREATE FUNCTION pride.reject_correction() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='corrected' THEN RAISE EXCEPTION 'synthetic failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_correction BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_correction()`)
	status(t, admin.call("PUT", path, Input{j.Records[0].Version, "present", nil, "atomic test"}), 503)
	after := parse[Journal](t, admin.call("GET", f.adminPath(j.Session.ID), nil), 200)
	if after.Records[0].Version != j.Records[0].Version || *after.Records[0].State != "absent" || after.Records[0].ChangeReason != nil || f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1`, j.Records[0].ID) != 1 {
		t.Fatal("partial correction committed")
	}
	f.exec(t, `DROP TRIGGER reject_correction ON pride.audit_events;DROP FUNCTION pride.reject_correction()`)
	// DB guards retain snapshot/terminal invariants after extending closed updates.
	for _, q := range []string{`UPDATE pride.attendance_records SET state='present',absence_reason=NULL WHERE id=$1`, `UPDATE pride.attendance_records SET origin='correction',change_reason='',version=version+1 WHERE id=$1`, `UPDATE pride.attendance_records SET club_membership_id=gen_random_uuid(),origin='correction',change_reason='reason',version=version+1 WHERE id=$1`, `DELETE FROM pride.attendance_records WHERE id=$1`} {
		if _, e := f.pool.Exec(context.Background(), q, j.Records[0].ID); e == nil {
			t.Fatal("SQL guard allowed invalid closed update")
		}
	}
	open := f.open(t, f.newSession(t, f.group, "2002-02-01"))
	status(t, admin.call("PUT", f.adminPath(open.Session.ID)+"/records/"+open.Records[0].ID, Input{1, "present", nil, "reason"}), 409)
	if _, e := f.pool.Exec(context.Background(), `UPDATE pride.attendance_records SET state='present',origin='correction',change_reason='reason',marked_by=$2,marked_at=now(),version=version+1 WHERE id=$1`, open.Records[0].ID, f.trainerID); e == nil {
		t.Fatal("correction allowed in open journal")
	}
	// Another record/session must never be addressed through this session URL.
	status(t, admin.call("GET", f.adminPath(open.Session.ID)+"/records/"+j.Records[0].ID+"/history", nil), 404)
}
func TestLiveCorrectionAccess(t *testing.T) {
	f := setup(t)
	f.member(t, "Synthetic", "2000-01-01", nil)
	admin, id := f.administrator(t, "admin", f.club)
	j := f.closed(t, "2000-01-02")
	path := f.adminPath(j.Session.ID)
	target := path + "/records/" + j.Records[0].ID
	f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, id)
	status(t, admin.call("GET", path, nil), 404)
	status(t, admin.call("PUT", target, Input{2, "present", nil, "reason"}), 404)
	status(t, admin.call("GET", target+"/history", nil), 404)
	f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, id, f.club)
	f.exec(t, `UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, id)
	status(t, admin.call("GET", path, nil), 401)
}

func TestCorrectionRechecksAccessAfterMiddleware(t *testing.T) {
	for _, mode := range []string{"role", "session", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			f.member(t, "Synthetic", "2000-01-01", nil)
			admin, id := f.administrator(t, "admin", f.club)
			j := f.closed(t, "2000-03-01")
			access, e := auth.New(f.pool, auth.Config{Origin: origin})
			if e != nil {
				t.Fatal(e)
			}
			service := New(f.pool, access)
			domain := http.NewServeMux()
			domain.Handle("PUT /api/clubs/{clubID}/journals/{sessionID}/records/{recordID}", access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "role":
					f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, id)
				case "session":
					f.exec(t, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1`, id)
				case "disabled":
					f.exec(t, `UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, id)
				}
				service.correct(w, r)
			})))
			admin.h = domain
			status(t, admin.call("PUT", f.adminPath(j.Session.ID)+"/records/"+j.Records[0].ID, Input{j.Records[0].Version, "present", nil, "must be denied"}), 404)
			if f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1 AND action='corrected'`, j.Records[0].ID) != 0 {
				t.Fatal("revoked actor committed correction")
			}
		})
	}
}
