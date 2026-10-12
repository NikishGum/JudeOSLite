package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"

	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"sync"
)

const password = "synthetic-schedule-password"
const origin = "https://schedule.example.test"

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL participants tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_schedule_%d", time.Now().UnixNano())
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

func TestScheduleIntegration(t *testing.T) {
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
	now := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	h := access.Handler(s.Handler(groups.New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler()))))
	owner := login(t, h, "owner")
	a := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "A", "timezone": "Europe/Minsk"}), 201)
	b := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "PRIVATE B", "timezone": "UTC"}), 201)
	staff := func(name, club, role string) string {
		var id string
		e := pool.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name).Scan(&id)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, id, club, role); e != nil {
			t.Fatal(e)
		}
		return id
	}
	staff("admin-a", a.ID, "administrator")
	staff("admin-b", b.ID, "administrator")
	trainerID := staff("trainer-a", a.ID, "trainer")
	otherTrainer := staff("trainer-b", b.ID, "trainer")
	ca, cb, trainer := login(t, h, "admin-a"), login(t, h, "admin-b"), login(t, h, "trainer-a")
	ga := "/api/clubs/" + a.ID + "/groups"
	gb := "/api/clubs/" + b.ID + "/groups"
	base := "/api/clubs/" + a.ID + "/schedule"
	group := parse[groups.Group](t, ca.call("POST", ga, map[string]any{"name": "A group"}), 201)
	foreign := parse[groups.Group](t, cb.call("POST", gb, map[string]any{"name": "PRIVATE B group"}), 201)
	input := Input{GroupID: group.ID, TrainerID: trainerID, Mode: "day_parity", Parity: "odd", Time: "18:00", Duration: 60, From: "2028-01-01"}
	preview := func(path string, in Input) Preview {
		return parse[Preview](t, ca.call("POST", path+"/preview", in), 200)
	}
	save := func(in Input) Rule {
		p := preview(base, in)
		in.PreviewToken = p.Token
		return parse[Rule](t, ca.call("POST", base, in), 201)
	}
	sessions := func(from, until string) []Session {
		return parse[[]Session](t, ca.call("GET", base+"/sessions?from="+from+"&until="+until, nil), 200)
	}
	count := func(query string, args ...any) int {
		var n int
		if e := pool.QueryRow(ctx, query, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	t.Run("rights validation csrf and cross club", func(t *testing.T) {
		status(t, client{h: h}.call("GET", base, nil), 401)
		status(t, cb.call("GET", base, nil), 404)
		status(t, trainer.call("POST", base+"/preview", input), 404)
		bad := input
		bad.GroupID = foreign.ID
		status(t, ca.call("POST", base+"/preview", bad), 404)
		bad = input
		bad.TrainerID = otherTrainer
		status(t, ca.call("POST", base+"/preview", bad), 404)
		noCSRF := ca
		noCSRF.csrf = ""
		status(t, noCSRF.call("POST", base+"/preview", input), 403)
		status(t, ca.call("POST", base+"/preview", map[string]any{"role": "owner"}), 400)
		bad = input
		bad.Until = &bad.From
		status(t, ca.call("POST", base+"/preview", bad), 400)
		status(t, ca.call("POST", base, input), 409)
	})
	p := preview(base, input)
	rule := save(input)
	t.Run("preview exact dates and idempotent simultaneous generation", func(t *testing.T) {
		all := sessions("2028-01-01", "2028-03-01")
		if len(all) != len(p.Dates) || len(all) != 31 {
			t.Fatalf("preview mismatch %d %d", len(all), len(p.Dates))
		}
		for i, v := range all {
			if v.Date != p.Dates[i].Date || !v.StartsAt.Equal(p.Dates[i].StartsAt) || !v.EndsAt.Equal(p.Dates[i].EndsAt) {
				t.Fatal("preview not same as persisted")
			}
		}
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for range 4 {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- s.Generate(ctx, now) }()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatal(e)
			}
		}
		if len(sessions("2028-01-01", "2028-03-01")) != len(all) {
			t.Fatal("duplicate generation")
		}
		next := time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)
		if e := s.Generate(ctx, next); e != nil {
			t.Fatal(e)
		}
		if e := New(pool, access).Generate(ctx, next); e != nil {
			t.Fatal(e)
		}
		march := sessions("2028-03-01", "2028-04-01")
		if len(march) != 16 {
			t.Fatalf("rollover %d", len(march))
		}
		if count(`SELECT count(*) FROM pride.training_sessions WHERE superseded_at IS NULL`) != 47 {
			t.Fatal("restart/rollover duplicates")
		}
	})
	t.Run("far selected dates once duplicates and simultaneous submit", func(t *testing.T) {
		selected := input
		selected.Mode = "date_list"
		selected.Parity = ""
		selected.Dates = []string{"2028-07-31", "2030-02-28"}
		selected.From = "2028-07-31"
		selected.Time = "19:00"
		save(selected)
		if len(sessions("2030-02-01", "2030-03-01")) != 1 {
			t.Fatal("far date absent")
		}
		once := selected
		once.Mode = "once"
		once.Dates = []string{"2028-06-30"}
		once.From = once.Dates[0]
		p := preview(base, once)
		once.PreviewToken = p.Token
		var wg sync.WaitGroup
		out := make(chan int, 2)
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); out <- ca.call("POST", base, once).Code }()
		}
		wg.Wait()
		close(out)
		counts := map[int]int{}
		for code := range out {
			counts[code]++
		}
		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("submit race %v", counts)
		}
		one := sessions("2028-06-30", "2028-07-01")
		if len(one) != 1 || one[0].RuleID != nil {
			t.Fatal("once got rule")
		}
		status(t, ca.call("POST", base+"/preview", once), 409)
		past := once
		past.From = "2027-12-31"
		past.Dates = []string{past.From}
		status(t, ca.call("POST", base+"/preview", past), 400)
	})
	t.Run("edits keep exceptions open closed frozen past IDs and snapshots", func(t *testing.T) {
		all := sessions("2028-01-01", "2028-02-01")
		exceptions := all[:5]
		updates := []string{`status='cancelled',cancelled_at=now(),cancellation_reason='synthetic'`, `is_manual_override=true,trainer_id='` + trainerID + `',starts_at=starts_at+interval '15 minutes'`, `status='open',roster_frozen_at=now()`, `status='closed',roster_frozen_at=now(),closed_at=now()`, `roster_frozen_at=now()`}
		for i, v := range exceptions {
			if _, e := pool.Exec(ctx, `UPDATE pride.training_sessions SET `+updates[i]+`,version=version+1 WHERE id=$1`, v.ID); e != nil {
				t.Fatal(e)
			}
		}
		if _, e := pool.Exec(ctx, `CREATE TABLE pride.test_snapshot(session_id uuid PRIMARY KEY REFERENCES pride.training_sessions(id),value text)`); e != nil {
			t.Fatal(e)
		}
		if _, e := pool.Exec(ctx, `INSERT INTO pride.test_snapshot VALUES($1,'open/closed roster retained')`, exceptions[2].ID); e != nil {
			t.Fatal(e)
		}
		before := sessions("2028-01-01", "2028-01-10")
		change := input
		change.Version = rule.Version
		change.From = "2028-01-02"
		change.Time = "20:00"
		change.Parity = "even"
		p := preview(base+"/"+rule.ID, change)
		if len(p.Preserved) != 4 || len(p.Affected) == 0 {
			t.Fatalf("preserved %d affected %d", len(p.Preserved), len(p.Affected))
		}
		change.PreviewToken = p.Token
		newRule := parse[Rule](t, ca.call("PUT", base+"/"+rule.ID, change), 201)
		if newRule.SeriesID != rule.SeriesID || newRule.ID == rule.ID {
			t.Fatal("rule history overwritten")
		}
		old := parse[[]Rule](t, ca.call("GET", base, nil), 200)
		found := false
		for _, r := range old {
			if r.ID == rule.ID {
				found = r.Until != nil && *r.Until == change.From && r.Version == 2
			}
		}
		if !found {
			t.Fatal("old version not ended")
		}
		after := sessions("2028-01-01", "2028-01-10")
		for _, v := range before {
			present := false
			for _, n := range after {
				if n.ID == v.ID {
					present = reflect.DeepEqual(n, v)
				}
			}
			if !present {
				t.Fatalf("exception/past rewritten %+v", v)
			}
		}
		for _, v := range p.Affected {
			if count(`SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND superseded_at IS NOT NULL`, v.ID) != 1 {
				t.Fatal("lost retired ID")
			}
		}
		if count(`SELECT count(*) FROM pride.test_snapshot`) != 1 {
			t.Fatal("snapshot lost")
		}
		if e := s.Generate(ctx, now); e != nil {
			t.Fatal(e)
		}
		after2 := sessions("2028-01-01", "2028-02-01")
		if len(after2) != len(after)+len(sessions("2028-01-10", "2028-02-01")) {
			t.Fatal("regeneration duplicate")
		}
		for _, v := range exceptions {
			if count(`SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND superseded_at IS NULL`, v.ID) != 1 {
				t.Fatal("exception resurrected")
			}
		}
		status(t, ca.call("PUT", base+"/"+rule.ID, change), 409)
		rule = newRule
		input = change
		input.Version = 1
		input.PreviewToken = ""
	})
	t.Run("preview change and atomic audit rollback", func(t *testing.T) {
		change := input
		change.From = "2028-01-10"
		change.Time = "21:00"
		p := preview(base+"/"+rule.ID, change)
		change.PreviewToken = p.Token
		if len(p.Affected) == 0 {
			t.Fatal("no affected")
		}
		v := p.Affected[0]
		if _, e := pool.Exec(ctx, `UPDATE pride.training_sessions SET is_manual_override=true,version=version+1 WHERE id=$1`, v.ID); e != nil {
			t.Fatal(e)
		}
		status(t, ca.call("PUT", base+"/"+rule.ID, change), 409)
		p = preview(base+"/"+rule.ID, change)
		change.PreviewToken = p.Token
		before := count(`SELECT count(*) FROM pride.schedule_rules`)
		retired := count(`SELECT count(*) FROM pride.training_sessions WHERE superseded_at IS NOT NULL`)
		if _, e := pool.Exec(ctx, `CREATE FUNCTION pride.reject_schedule_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='schedule_rule' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_schedule_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_schedule_audit()`); e != nil {
			t.Fatal(e)
		}
		status(t, ca.call("PUT", base+"/"+rule.ID, change), 503)
		if before != count(`SELECT count(*) FROM pride.schedule_rules`) || retired != count(`SELECT count(*) FROM pride.training_sessions WHERE superseded_at IS NOT NULL`) {
			t.Fatal("partial rollback")
		}
		if _, e := pool.Exec(ctx, `DROP TRIGGER reject_schedule_audit ON pride.audit_events`); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("same-date exceptions survive changed time and repeated versions", func(t *testing.T) {
		change := input
		change.From = "2028-01-10"
		change.Time = "22:00"
		p := preview(base+"/"+rule.ID, change)
		if len(p.Affected) < 2 {
			t.Fatal("missing future plans")
		}
		cancelled, override := p.Affected[0], p.Affected[1]
		if _, e := pool.Exec(ctx, `UPDATE pride.training_sessions SET status='cancelled',cancelled_at=now(),version=version+1 WHERE id=$1`, cancelled.ID); e != nil {
			t.Fatal(e)
		}
		if _, e := pool.Exec(ctx, `UPDATE pride.training_sessions SET is_manual_override=true,starts_at=starts_at+interval '15 minutes',version=version+1 WHERE id=$1`, override.ID); e != nil {
			t.Fatal(e)
		}
		p = preview(base+"/"+rule.ID, change)
		for _, o := range p.Dates {
			if o.Date == cancelled.Date || o.Date == override.Date {
				t.Fatal("exception would be recreated at new time")
			}
		}
		change.PreviewToken = p.Token
		rule = parse[Rule](t, ca.call("PUT", base+"/"+rule.ID, change), 201)
		input = change
		input.PreviewToken = ""
		input.Version = 1
		for range 2 {
			if e := s.Generate(ctx, now); e != nil {
				t.Fatal(e)
			}
		}
		for _, v := range []Session{cancelled, override} {
			if count(`SELECT count(*) FROM pride.training_sessions WHERE group_id=$1 AND starts_on=$2::date AND superseded_at IS NULL`, group.ID, v.Date) != 1 {
				t.Fatal("exception recreated")
			}
		}
	})
	t.Run("live administrator checked inside transaction after middleware", func(t *testing.T) {
		var adminID string
		if e := pool.QueryRow(ctx, `SELECT id::text FROM pride.staff_users WHERE login='admin-a'`).Scan(&adminID); e != nil {
			t.Fatal(e)
		}
		wrapped := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.SetPathValue("clubID", a.ID)
			if _, e := pool.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, adminID); e != nil {
				t.Fatal(e)
			}
			s.save(true)(w, r)
		}))
		stale := ca
		stale.h = wrapped
		status(t, stale.call("POST", base+"/preview", input), 404)
		if _, e := pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role)VALUES($1,$2,'administrator')`, adminID, a.ID); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("future one-off conflict preserves slot and extends remaining horizon", func(t *testing.T) {
		once := input
		once.Mode = "once"
		once.Parity = ""
		once.Dates = []string{"2028-04-02"}
		once.From = once.Dates[0]
		once.Version = 0
		p := preview(base, once)
		once.PreviewToken = p.Token
		v := parse[Session](t, ca.call("POST", base, once), 201)
		if e := s.Generate(ctx, time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(e, ErrDuplicate) {
			t.Fatalf("expected duplicate warning: %v", e)
		}
		if count(`SELECT count(*) FROM pride.training_sessions WHERE group_id=$1 AND starts_on='2028-04-02' AND superseded_at IS NULL`, group.ID) != 1 {
			t.Fatal("occupied slot duplicated")
		}
		if count(`SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND schedule_rule_id IS NULL`, v.ID) != 1 {
			t.Fatal("one-off overwritten")
		}
		if len(sessions("2028-04-01", "2028-05-01")) != 15 {
			t.Fatal("conflict prevented other dates")
		}
	})
	t.Run("trainer own assignments only revoke active lifecycle", func(t *testing.T) {
		status(t, trainer.call("GET", base, nil), 200)
		status(t, trainer.call("GET", base+"/sessions?from=2028-01-01&until=2028-03-01", nil), 200)
		before := count(`SELECT count(*) FROM pride.training_sessions`)
		if _, e := pool.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='trainer'`, trainerID); e != nil {
			t.Fatal(e)
		}
		status(t, trainer.call("GET", base, nil), 404)
		status(t, ca.call("POST", base+"/preview", input), 404)
		if e := s.Generate(ctx, time.Date(2028, 4, 1, 0, 0, 0, 0, time.UTC)); e != nil {
			t.Fatal(e)
		}
		if before != count(`SELECT count(*) FROM pride.training_sessions`) {
			t.Fatal("revoked trainer got sessions")
		}
		if _, e := pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, trainerID, a.ID); e != nil {
			t.Fatal(e)
		}
		if _, e := pool.Exec(ctx, `UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, group.ID); e != nil {
			t.Fatal(e)
		}
		status(t, ca.call("POST", base+"/preview", input), 409)
		if e := s.Generate(ctx, time.Date(2028, 4, 1, 0, 0, 0, 0, time.UTC)); e != nil {
			t.Fatal(e)
		}
		if before != count(`SELECT count(*) FROM pride.training_sessions`) {
			t.Fatal("archived group got sessions")
		}
		if _, e := pool.Exec(ctx, `UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, a.ID); e != nil {
			t.Fatal(e)
		}
		status(t, ca.call("POST", base+"/preview", input), 409)
		status(t, ca.call("GET", base, nil), 200)
	})
}
