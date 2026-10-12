package sessions

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
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
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
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL sessions tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("judeos_sessions_%d", time.Now().UnixNano())
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

func TestSessionsIntegration(t *testing.T) {
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
	generator := schedule.New(pool, access)
	h := access.Handler(s.Handler(generator.Handler(groups.New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler())))))
	owner := login(t, h, "owner")
	a := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "A", "timezone": "Europe/Minsk"}), 201)
	b := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "PRIVATE B", "timezone": "UTC"}), 201)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, query, args...); e != nil {
			t.Fatal(e)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if e := pool.QueryRow(ctx, query, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	staff := func(name, club, role string) string {
		var id string
		if e := pool.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name).Scan(&id); e != nil {
			t.Fatal(e)
		}
		exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, id, club, role)
		return id
	}
	adminID := staff("admin-a", a.ID, "administrator")
	staff("admin-b", b.ID, "administrator")
	trainerID := staff("trainer-a", a.ID, "trainer")
	replacement := staff("replacement-a", a.ID, "trainer")
	foreignTrainer := staff("private-trainer-b", b.ID, "trainer")
	ca, cb, trainer, newTrainer := login(t, h, "admin-a"), login(t, h, "admin-b"), login(t, h, "trainer-a"), login(t, h, "replacement-a")
	group := parse[groups.Group](t, ca.call("POST", "/api/clubs/"+a.ID+"/groups", map[string]string{"name": "A group"}), 201)
	foreignGroup := parse[groups.Group](t, cb.call("POST", "/api/clubs/"+b.ID+"/groups", map[string]string{"name": "PRIVATE group"}), 201)
	base := "/api/clubs/" + a.ID + "/sessions"
	rules := "/api/clubs/" + a.ID + "/schedule"
	input := createInput{GroupID: group.ID, TrainerID: trainerID, Date: "2028-01-02", Time: "18:00", Duration: 60}
	create := func(date string) Session {
		t.Helper()
		in := input
		in.Date = date
		return parse[Session](t, ca.call("POST", base, in), 201)
	}
	read := func(id string) Session {
		t.Helper()
		v, e := scan(pool.QueryRow(ctx, `SELECT `+columns+joins+`WHERE s.id::text=$1`, id))
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	query := base + "?from=2028-01-01&until=2028-02-01"
	command := func(v Session, action string, in commandInput, want int) *httptest.ResponseRecorder {
		t.Helper()
		in.Version = v.Version
		w := ca.call("POST", base+"/"+v.ID+"/"+action, in)
		status(t, w, want)
		return w
	}
	t.Run("one off validation isolation and no rule", func(t *testing.T) {
		v := create(input.Date)
		if v.RuleID != nil || v.SourceDate != input.Date || v.Date != input.Date || v.TrainerName != "trainer-a" {
			t.Fatalf("one off %+v", v)
		}
		status(t, ca.call("POST", base, input), 409)
		status(t, cb.call("GET", query, nil), 404)
		status(t, client{h: h}.call("GET", query, nil), 401)
		status(t, trainer.call("POST", base, input), 404)
		for _, in := range []createInput{{foreignGroup.ID, trainerID, "2028-01-03", "18:00", 60}, {group.ID, foreignTrainer, "2028-01-03", "18:00", 60}} {
			status(t, ca.call("POST", base, in), 404)
		}
		for _, in := range []createInput{{group.ID, trainerID, "2027-12-31", "18:00", 60}, {group.ID, trainerID, "2028-02-30", "18:00", 60}, {group.ID, trainerID, "2028-01-03", "24:00", 60}, {group.ID, trainerID, "2028-01-03", "18:00", 1441}} {
			status(t, ca.call("POST", base, in), 400)
		}
		bad := ca
		bad.csrf = "bad"
		status(t, bad.call("POST", base, input), 403)
		status(t, ca.call("POST", base, map[string]any{"isOwner": true}), 400)
		status(t, ca.call("GET", query+"&groupId="+foreignGroup.ID, nil), 404)
		for _, g := range parse[[]groups.Group](t, trainer.call("GET", base+"/groups", nil), 200) {
			if g.ClubID != a.ID {
				t.Fatal("trainer group selector leaked foreign club")
			}
		}
		status(t, ca.call("GET", base+"?from=2028-01-01&until=2029-01-03", nil), 400)
		if count(`SELECT count(*) FROM pride.schedule_rules`) != 0 {
			t.Fatal("one off created a rule")
		}
	})
	t.Run("cancel planned reserves slot and records reason without marks", func(t *testing.T) {
		v := create("2028-01-03")
		command(v, "cancel", commandInput{}, 400)
		command(v, "cancel", commandInput{Reason: strings.Repeat("я", 1001)}, 400)
		out := parse[Session](t, command(v, "cancel", commandInput{Reason: "  synthetic private reason  "}, 200), 200)
		if out.Status != "cancelled" || out.CancelledAt == nil || out.ClosedAt != nil || out.Reason == nil || *out.Reason != "synthetic private reason" || out.Version != 2 {
			t.Fatalf("cancel %+v", out)
		}
		command(v, "replace-trainer", commandInput{TrainerID: replacement}, 409)
		command(out, "replace-trainer", commandInput{TrainerID: replacement}, 409)
		in := input
		in.Date = v.Date
		status(t, ca.call("POST", base, in), 409)
		if count(`SELECT count(*) FROM pride.training_sessions WHERE id=$1`, v.ID) != 1 {
			t.Fatal("cancel deleted session")
		}
	})
	t.Run("open cancellation and replacement preserve frozen roster and authors", func(t *testing.T) {
		v := create("2028-01-04")
		// A consumer fixture stands for V1-09's snapshot and marks, not production scaffolding.
		exec(`CREATE TABLE pride.test_session_history(session_id uuid REFERENCES pride.training_sessions(id),marked_by uuid REFERENCES pride.staff_users(id),value text)`)
		exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=now(),version=version+1 WHERE id=$1`, v.ID)
		exec(`INSERT INTO pride.test_session_history VALUES($1,$2,'frozen snapshot / present / unmarked')`, v.ID, trainerID)
		v = read(v.ID)
		command(v, "cancel", commandInput{Reason: "synthetic"}, 409)
		command(v, "reschedule", commandInput{Date: "2028-01-20", Time: "19:00", Duration: 60}, 409)
		out := parse[Session](t, command(v, "replace-trainer", commandInput{TrainerID: replacement}, 200), 200)
		if out.Status != "open" || !out.Frozen || !out.Manual || out.TrainerID != replacement || out.SourceDate != v.SourceDate {
			t.Fatal("replace damaged journal")
		}
		list := parse[[]Session](t, trainer.call("GET", query, nil), 200)
		for _, item := range list {
			if item.ID == v.ID {
				t.Fatal("old trainer kept actual session")
			}
		}
		list = parse[[]Session](t, newTrainer.call("GET", query, nil), 200)
		if len(list) != 1 || list[0].ID != v.ID {
			t.Fatal("replacement cannot see session")
		}
		status(t, trainer.call("POST", base+"/"+v.ID+"/replace-trainer", commandInput{Version: out.Version, TrainerID: trainerID}), 404)
		cancelled := parse[Session](t, command(out, "cancel", commandInput{Reason: "synthetic open cancel", ConfirmOpen: true}, 200), 200)
		if cancelled.ClosedAt == nil || !cancelled.Frozen || cancelled.Status != "cancelled" || !cancelled.StartsAt.Equal(v.StartsAt) {
			t.Fatal("open cancel did not close writing/preserve journal")
		}
		if count(`SELECT count(*) FROM pride.test_session_history WHERE session_id=$1 AND marked_by=$2 AND value='frozen snapshot / present / unmarked'`, v.ID, trainerID) != 1 {
			t.Fatal("marks/snapshot/author changed")
		}
	})
	t.Run("actual dates moves and regeneration retain original series occurrence", func(t *testing.T) {
		in := schedule.Input{GroupID: group.ID, TrainerID: trainerID, Mode: "date_list", Dates: []string{"2028-01-05", "2028-01-06", "2028-01-07"}, From: "2028-01-05", Time: "18:00", Duration: 60}
		p := parse[schedule.Preview](t, ca.call("POST", rules+"/preview", in), 200)
		in.PreviewToken = p.Token
		rule := parse[schedule.Rule](t, ca.call("POST", rules, in), 201)
		list := parse[[]Session](t, ca.call("GET", query, nil), 200)
		selected := []Session{}
		for _, v := range list {
			if v.RuleID != nil && *v.RuleID == rule.ID {
				selected = append(selected, v)
			}
		}
		if len(selected) != 3 {
			t.Fatal("rule sessions missing")
		}
		command(selected[0], "replace-trainer", commandInput{TrainerID: foreignTrainer}, 404)
		replaced := parse[Session](t, command(selected[0], "replace-trainer", commandInput{TrainerID: replacement}, 200), 200)
		moved := parse[Session](t, command(replaced, "reschedule", commandInput{Date: "2028-02-10", Time: "23:30", Duration: 90}, 200), 200)
		if moved.SourceDate != "2028-01-05" || moved.Date != "2028-02-10" || moved.RuleID == nil || *moved.RuleID != rule.ID {
			t.Fatal("source identity lost")
		}
		command(selected[1], "cancel", commandInput{Reason: "synthetic series cancel"}, 200)
		if e := generator.Generate(ctx, now); e != nil {
			t.Fatal(e)
		}
		if count(`SELECT count(*) FROM pride.training_sessions WHERE schedule_rule_id=$1 AND superseded_at IS NULL`, rule.ID) != 3 {
			t.Fatal("generation resurrected/multiplied exceptions")
		}
		feb := parse[[]Session](t, ca.call("GET", base+"?from=2028-02-10&until=2028-02-11&groupId="+group.ID, nil), 200)
		if len(feb) != 1 || feb[0].ID != moved.ID {
			t.Fatal("calendar not filtered by actual date")
		}
		// A dated time change preserves moved occurrence/cancel even outside calendar date.
		in.Version = rule.Version
		in.From = "2028-01-06"
		in.Dates = []string{"2028-01-06", "2028-01-07"}
		in.Time = "20:00"
		in.PreviewToken = ""
		p = parse[schedule.Preview](t, ca.call("POST", rules+"/"+rule.ID+"/preview", in), 200)
		in.PreviewToken = p.Token
		parse[schedule.Rule](t, ca.call("PUT", rules+"/"+rule.ID, in), 201)
		if e := generator.Generate(ctx, now); e != nil {
			t.Fatal(e)
		}
		if read(moved.ID).Version != moved.Version || count(`SELECT count(*) FROM pride.training_sessions WHERE group_id=$1 AND starts_on='2028-01-06' AND superseded_at IS NULL`, group.ID) != 1 {
			t.Fatal("dated edit rewrote exception")
		}
		var retired string
		if e := pool.QueryRow(ctx, `SELECT id::text FROM pride.training_sessions WHERE schedule_rule_id=$1 AND superseded_at IS NOT NULL`, rule.ID).Scan(&retired); e != nil {
			t.Fatal(e)
		}
		command(read(retired), "cancel", commandInput{Reason: "retired"}, 409)
		for _, v := range parse[[]Session](t, ca.call("GET", query, nil), 200) {
			if v.ID == retired || v.ID == moved.ID {
				t.Fatal("calendar contains retired/wrong actual date")
			}
		}
	})
	t.Run("closed journal and database identity guards", func(t *testing.T) {
		v := create("2028-01-08")
		exec(`UPDATE pride.training_sessions SET status='closed',roster_frozen_at=now(),closed_at=now(),version=version+1 WHERE id=$1`, v.ID)
		v = read(v.ID)
		command(v, "cancel", commandInput{Reason: "closed", ConfirmOpen: true}, 409)
		command(v, "replace-trainer", commandInput{TrainerID: replacement}, 409)
		command(v, "reschedule", commandInput{Date: "2028-01-20", Time: "19:00", Duration: 60}, 409)
		for _, change := range []string{`trainer_id='` + replacement + `'`, `starts_at=starts_at+interval '1 hour'`, `status='cancelled'`, `starts_on='2028-01-09'`, `roster_frozen_at=NULL`} {
			if _, e := pool.Exec(ctx, `UPDATE pride.training_sessions SET `+change+` WHERE id=$1`, v.ID); e == nil {
				t.Fatalf("DB accepted %s", change)
			}
		}
	})
	t.Run("concurrent commands creation generation and version conflict", func(t *testing.T) {
		v := create("2028-01-09")
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for _, action := range []string{"cancel", "replace-trainer"} {
			wg.Add(1)
			go func(action string) {
				defer wg.Done()
				in := commandInput{Version: v.Version}
				if action == "cancel" {
					in.Reason = "concurrent"
				} else {
					in.TrainerID = replacement
				}
				codes <- ca.call("POST", base+"/"+v.ID+"/"+action, in).Code
			}(action)
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatalf("command race %v", counts)
		}
		in := input
		in.Date = "2028-01-10"
		codes = make(chan int, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- ca.call("POST", base, in).Code }()
		}
		wg.Wait()
		close(codes)
		counts = map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("creation race %v", counts)
		}
		// Lock serializes generator and cancellation; neither rewrites the exception.
		v = create("2028-01-11")
		errs := make(chan error, 1)
		wg.Add(1)
		go func() { defer wg.Done(); errs <- generator.Generate(ctx, now) }()
		command(v, "cancel", commandInput{Reason: "generation race"}, 200)
		wg.Wait()
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
		if read(v.ID).Status != "cancelled" {
			t.Fatal("generation erased cancellation")
		}
	})
	t.Run("duplicate move and audit failure roll back all commands", func(t *testing.T) {
		v := create("2028-01-12")
		target := create("2028-01-13")
		command(v, "reschedule", commandInput{Date: target.Date, Time: "18:00", Duration: 60}, 409)
		exec(`CREATE FUNCTION pride.reject_session_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='training_session' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_session_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_session_audit()`)
		for action, in := range map[string]commandInput{"cancel": {Reason: "audit rollback"}, "replace-trainer": {TrainerID: replacement}, "reschedule": {Date: "2028-01-14", Time: "19:00", Duration: 60}} {
			command(v, action, in, 503)
			after := read(v.ID)
			if after.Version != v.Version || after.Status != v.Status || after.TrainerID != v.TrainerID || !after.StartsAt.Equal(v.StartsAt) {
				t.Fatal("audit failure committed session")
			}
		}
		in := input
		in.Date = "2028-01-14"
		status(t, ca.call("POST", base, in), 503)
		if count(`SELECT count(*) FROM pride.training_sessions WHERE starts_on='2028-01-14'`) > 0 {
			t.Fatal("audit failure committed creation")
		}
		exec(`DROP TRIGGER reject_session_audit ON pride.audit_events`)
		if count(`SELECT count(*) FROM pride.audit_events WHERE entity_type='training_session' AND (before_state::text||after_state::text) LIKE '%synthetic private reason%'`) != 0 {
			t.Fatal("free text leaked into audit")
		}
	})
	t.Run("lifecycle and live actor target guards", func(t *testing.T) {
		v := create("2028-01-15")
		exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, replacement)
		command(v, "replace-trainer", commandInput{TrainerID: replacement}, 404)
		exec(`UPDATE pride.staff_users SET disabled_at=NULL WHERE id=$1`, replacement)
		exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2`, replacement, a.ID)
		command(v, "replace-trainer", commandInput{TrainerID: replacement}, 404)
		exec(`UPDATE pride.training_groups SET archived_at=now() WHERE id=$1`, group.ID)
		command(v, "replace-trainer", commandInput{TrainerID: trainerID}, 409)
		command(v, "cancel", commandInput{Reason: "archived group still can cancel existing plan"}, 200)
		exec(`UPDATE pride.training_groups SET archived_at=NULL WHERE id=$1`, group.ID)
		v = create("2028-01-16")
		exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, a.ID)
		command(v, "cancel", commandInput{Reason: "archived club"}, 409)
		parse[[]Session](t, ca.call("GET", query, nil), 200)
		exec(`UPDATE pride.clubs SET archived_at=NULL WHERE id=$1`, a.ID)
		// Revoke between HTTP authentication and command transaction.
		guarded := access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2`, adminID, a.ID)
			r.SetPathValue("clubID", a.ID)
			r.SetPathValue("sessionID", v.ID)
			s.command("cancel")(w, r)
		}))
		stale := ca
		stale.h = guarded
		status(t, stale.call("POST", base, commandInput{Version: v.Version, Reason: "revoke race"}), 404)
		if read(v.ID).Version != v.Version {
			t.Fatal("revoked actor committed command")
		}
		exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, adminID, a.ID)
		guarded = access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exec(`UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1`, adminID)
			r.SetPathValue("clubID", a.ID)
			r.SetPathValue("sessionID", v.ID)
			s.command("cancel")(w, r)
		}))
		stale.h = guarded
		status(t, stale.call("POST", base, commandInput{Version: v.Version, Reason: "live session revoke"}), 404)
		if read(v.ID).Version != v.Version {
			t.Fatal("revoked session committed command")
		}
	})
}
