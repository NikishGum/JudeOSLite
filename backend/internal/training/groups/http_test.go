package groups_test

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
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-groups-password"
const origin = "https://groups.example.test"

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
	name := fmt.Sprintf("judeos_groups_%d", time.Now().UnixNano())
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

func cardInput(initial *groups.InitialGroup) any {
	return map[string]any{"identity": map[string]any{"fullName": "Синтетический Участник", "birthDate": "2014-02-28", "version": 0}, "joinedOn": "2025-09-01", "contactKind": "phone", "contactValue": "+375291234567", "version": 0, "initialGroup": initial}
}
func TestGroupsIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	if e := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); e != nil {
		t.Fatal(e)
	}
	access, e := auth.New(pool, auth.Config{Origin: origin})
	if e != nil {
		t.Fatal(e)
	}
	service := groups.New(pool, access)
	h := access.Handler(service.Handler(participants.New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler()))))
	owner := login(t, h, "owner")
	a := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "Synthetic A", "timezone": "Europe/Minsk"}), 201)
	b := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "Synthetic B", "timezone": "UTC"}), 201)
	for _, name := range []string{"admin-a", "admin-b", "trainer"} {
		club := a.ID
		role := "administrator"
		if name == "admin-b" {
			club = b.ID
		}
		if name == "trainer" {
			role = "trainer"
		}
		var id string
		if e := pool.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name).Scan(&id); e != nil {
			t.Fatal(e)
		}
		if _, e := pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role)VALUES($1,$2,$3)`, id, club, role); e != nil {
			t.Fatal(e)
		}
	}
	ca, cb, trainer := login(t, h, "admin-a"), login(t, h, "admin-b"), login(t, h, "trainer")
	ga, gb := "/api/clubs/"+a.ID+"/groups", "/api/clubs/"+b.ID+"/groups"
	pa, pb := "/api/clubs/"+a.ID+"/participants", "/api/clubs/"+b.ID+"/participants"
	g1 := parse[groups.Group](t, ca.call("POST", ga, map[string]any{"name": "Первый состав", "version": 0}), 201)
	g2 := parse[groups.Group](t, ca.call("POST", ga, map[string]any{"name": "Второй состав", "version": 0}), 201)
	foreign := parse[groups.Group](t, cb.call("POST", gb, map[string]any{"name": "PRIVATE B", "version": 0}), 201)
	card := parse[participants.Card](t, ca.call("POST", pa, cardInput(&groups.InitialGroup{GroupID: g1.ID, From: "2025-09-01"})), 201)
	memberBase := pa + "/" + card.ID + "/groups"
	initial := parse[[]groups.Interval](t, ca.call("GET", memberBase, nil), 200)[0]
	if initial.MembershipID != card.ID || initial.MembershipID == card.ParticipantID {
		t.Fatal("wrong card identity")
	}
	roster := func(group, date string) []groups.Interval {
		return parse[[]groups.Interval](t, ca.call("GET", ga+"/"+group+"/roster?date="+date, nil), 200)
	}
	t.Run("access csrf scope and validation", func(t *testing.T) {
		for _, path := range []string{ga, ga + "/" + g1.ID + "/roster?date=2025-09-01", memberBase} {
			status(t, client{h: h}.call("GET", path, nil), 401)
			status(t, cb.call("GET", path, nil), 404)
			status(t, trainer.call("GET", path, nil), 404)
		}
		status(t, ca.call("POST", gb, map[string]any{"name": "x"}), 404)
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": foreign.ID, "from": "2025-10-01"}), 404)
		status(t, ca.call("POST", pa+"/"+card.ParticipantID+"/groups", map[string]any{"groupId": g2.ID, "from": "2026-10-01"}), 404)
		status(t, ca.call("POST", ga, map[string]any{"name": "x", "role": "owner"}), 400)
		noCSRF := ca
		noCSRF.csrf = ""
		status(t, noCSRF.call("POST", ga, map[string]any{"name": "x"}), 403)
		for _, date := range []string{"2025-02-29", "2025-09", "0000-01-01", "2025-13-01"} {
			status(t, ca.call("GET", ga+"/"+g1.ID+"/roster?date="+date, nil), 400)
			status(t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": date}), 400)
		}
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": "2025-08-31", "until": "2025-09-01"}), 400)
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": "2025-09-01", "until": "2025-09-01"}), 400)
		local := parse[[]groups.Group](t, ca.call("GET", ga+"?state=all", nil), 200)
		raw, _ := json.Marshal(local)
		if strings.Contains(string(raw), "PRIVATE B") {
			t.Fatal("foreign group leaked")
		}
		status(t, owner.call("GET", gb, nil), 200)
	})
	t.Run("transfer boundaries return exclusion and stale versions", func(t *testing.T) {
		if len(roster(g1.ID, "2025-08-31")) != 0 || len(roster(g1.ID, "2025-09-01")) != 1 {
			t.Fatal("start boundary")
		}
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g1.ID, "from": "2025-10-01"}), 409)
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": "2025-10-01"}), 409)
		status(t, ca.call("POST", memberBase+"/"+initial.ID+"/transfer", map[string]any{"groupId": g2.ID, "date": "2025-09-01", "version": 1}), 400)
		next := parse[groups.Interval](t, ca.call("POST", memberBase+"/"+initial.ID+"/transfer", map[string]any{"groupId": g2.ID, "date": "2025-10-01", "version": 1}), 200)
		if len(roster(g1.ID, "2025-09-30")) != 1 || len(roster(g1.ID, "2025-10-01")) != 0 || len(roster(g2.ID, "2025-10-01")) != 1 {
			t.Fatal("transfer boundary")
		}
		status(t, ca.call("POST", memberBase+"/"+initial.ID+"/exclude", map[string]any{"date": "2025-09-15", "version": 1}), 409)
		back := parse[groups.Interval](t, ca.call("POST", memberBase+"/"+next.ID+"/transfer", map[string]any{"groupId": g1.ID, "date": "2025-11-01", "version": 1}), 200)
		if back.ID == initial.ID || len(roster(g1.ID, "2025-11-01")) != 1 || len(roster(g2.ID, "2025-10-31")) != 1 {
			t.Fatal("return rewrote old interval")
		}
		parse[groups.Interval](t, ca.call("POST", memberBase+"/"+back.ID+"/exclude", map[string]any{"date": "2025-12-01", "version": 1}), 200)
		if len(roster(g1.ID, "2025-11-30")) != 1 || len(roster(g1.ID, "2025-12-01")) != 0 {
			t.Fatal("exclusion boundary")
		}
		history := parse[[]groups.Interval](t, ca.call("GET", memberBase, nil), 200)
		if len(history) != 3 || history[0].Until == nil || *history[0].Until != "2025-10-01" {
			t.Fatal("history missing")
		}
	})
	t.Run("bounded interval transfer preserves original end", func(t *testing.T) {
		c := parse[participants.Card](t, ca.call("POST", pa, cardInput(nil)), 201)
		base := pa + "/" + c.ID + "/groups"
		i := parse[groups.Interval](t, ca.call("POST", base, map[string]any{"groupId": g1.ID, "from": "2026-01-01", "until": "2026-02-01"}), 201)
		n := parse[groups.Interval](t, ca.call("POST", base+"/"+i.ID+"/transfer", map[string]any{"groupId": g2.ID, "date": "2026-01-15", "version": 1}), 200)
		if n.Until == nil || *n.Until != "2026-02-01" {
			t.Fatal("bounded end lost")
		}
		parse[groups.Interval](t, ca.call("POST", base, map[string]any{"groupId": g1.ID, "from": "2026-02-01"}), 201)
	})
	t.Run("concurrent add and transfer serialize", func(t *testing.T) {
		c := parse[participants.Card](t, ca.call("POST", pa, cardInput(nil)), 201)
		base := pa + "/" + c.ID + "/groups"
		var wg sync.WaitGroup
		responses := make(chan *httptest.ResponseRecorder, 2)
		for _, g := range []string{g1.ID, g2.ID} {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				responses <- ca.call("POST", base, map[string]any{"groupId": id, "from": "2026-03-01"})
			}(g)
		}
		wg.Wait()
		close(responses)
		counts := map[int]int{}
		for w := range responses {
			counts[w.Code]++
		}
		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("add race: %v", counts)
		}
		i := parse[[]groups.Interval](t, ca.call("GET", base, nil), 200)[0]
		target := g2.ID
		if i.GroupID == g2.ID {
			target = g1.ID
		}
		responses = make(chan *httptest.ResponseRecorder, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				responses <- ca.call("POST", base+"/"+i.ID+"/transfer", map[string]any{"groupId": target, "date": "2026-04-01", "version": 1})
			}()
		}
		wg.Wait()
		close(responses)
		counts = map[int]int{}
		for w := range responses {
			counts[w.Code]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatalf("transfer race: %v", counts)
		}
		history := parse[[]groups.Interval](t, ca.call("GET", base, nil), 200)
		if len(history) != 2 {
			t.Fatal("partial or duplicate transfer")
		}
	})
	t.Run("rollback entire card and transfer on audit failure", func(t *testing.T) {
		var before, after int
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.participants`).Scan(&before)
		status(t, ca.call("POST", pa, cardInput(&groups.InitialGroup{GroupID: foreign.ID, From: "2025-09-01"})), 404)
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.participants`).Scan(&after)
		if before != after {
			t.Fatal("partial foreign-group card")
		}
		c := parse[participants.Card](t, ca.call("POST", pa, cardInput(&groups.InitialGroup{GroupID: g1.ID, From: "2025-09-01"})), 201)
		base := pa + "/" + c.ID + "/groups"
		old := parse[[]groups.Interval](t, ca.call("GET", base, nil), 200)[0]
		_, e := pool.Exec(ctx, `CREATE FUNCTION pride.reject_group_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='group_membership' AND NEW.action='added' THEN RAISE EXCEPTION 'synthetic failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_group_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_group_audit()`)
		if e != nil {
			t.Fatal(e)
		}
		defer pool.Exec(ctx, `DROP TRIGGER reject_group_audit ON pride.audit_events;DROP FUNCTION pride.reject_group_audit()`)
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.participants`).Scan(&before)
		status(t, ca.call("POST", pa, cardInput(&groups.InitialGroup{GroupID: g1.ID, From: "2025-09-01"})), 503)
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.participants`).Scan(&after)
		if before != after {
			t.Fatal("partial failed audit card")
		}
		status(t, ca.call("POST", base+"/"+old.ID+"/transfer", map[string]any{"groupId": g2.ID, "date": "2025-10-01", "version": 1}), 503)
		history := parse[[]groups.Interval](t, ca.call("GET", base, nil), 200)
		if len(history) != 1 || history[0].Until != nil || history[0].Version != 1 {
			t.Fatal("partial failed transfer")
		}
	})
	t.Run("database constraints and historical snapshot reference", func(t *testing.T) {
		c := parse[participants.Card](t, cb.call("POST", pb, cardInput(nil)), 201)
		if _, e := pool.Exec(ctx, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2025-09-01')`, a.ID, g1.ID, c.ID); e == nil {
			t.Fatal("cross-club card allowed")
		}
		if _, e := pool.Exec(ctx, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2025-09-01')`, a.ID, foreign.ID, card.ID); e == nil {
			t.Fatal("cross-club group allowed")
		}
		if _, e := pool.Exec(ctx, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2025-09-01')`, a.ID, g2.ID, card.ID); e == nil {
			t.Fatal("direct overlap allowed")
		}
		if _, e := pool.Exec(ctx, `UPDATE pride.group_memberships SET valid_from='2025-09-02' WHERE id=$1`, initial.ID); e == nil {
			t.Fatal("historical interval keys mutated")
		}
		if _, e := pool.Exec(ctx, `UPDATE pride.club_memberships SET joined_on='2025-09-02' WHERE id=$1`, card.ID); e == nil {
			t.Fatal("entry moved past initial interval")
		}
		_, e := pool.Exec(ctx, `CREATE TABLE pride.synthetic_snapshot (session_state text,source uuid,club_id uuid,group_id uuid,member uuid,FOREIGN KEY(source,club_id,group_id,member) REFERENCES pride.group_memberships(id,club_id,group_id,club_membership_id) ON DELETE RESTRICT)`)
		if e != nil {
			t.Fatal(e)
		}
		for _, state := range []string{"open", "closed"} {
			if _, e := pool.Exec(ctx, `INSERT INTO pride.synthetic_snapshot VALUES($1,$2,$3,$4,$5)`, state, initial.ID, a.ID, g1.ID, card.ID); e != nil {
				t.Fatal(e)
			}
		}
		current := parse[[]groups.Interval](t, ca.call("GET", memberBase, nil), 200)[0]
		parse[groups.Interval](t, ca.call("POST", memberBase+"/"+current.ID+"/transfer", map[string]any{"groupId": g2.ID, "date": "2025-09-15", "version": current.Version}), 200)
		parse[participants.Card](t, ca.call("POST", pa+"/"+card.ID+"/archive", map[string]int{"version": card.Version}), 200)
		g1 = parse[groups.Group](t, ca.call("POST", ga+"/"+g1.ID+"/archive", map[string]int{"version": g1.Version}), 200)
		if len(roster(g1.ID, "2025-09-30")) == 0 {
			t.Fatal("archive erased dated roster")
		}
		var n int
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.synthetic_snapshot WHERE source=$1 AND group_id=$2 AND member=$3`, initial.ID, g1.ID, card.ID).Scan(&n)
		if n != 2 {
			t.Fatal("snapshot rewritten")
		}
		if _, e := pool.Exec(ctx, `DELETE FROM pride.group_memberships WHERE id=$1`, initial.ID); e == nil {
			t.Fatal("snapshot source deleted")
		}
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": "2026-05-01"}), 409)
		restored := parse[participants.Card](t, ca.call("POST", pa+"/"+card.ID+"/restore", map[string]int{"version": card.Version + 1}), 200)
		if restored.ID != card.ID {
			t.Fatal("restore changed card")
		}
		status(t, ca.call("POST", memberBase, map[string]any{"groupId": g1.ID, "from": "2026-05-01"}), 409)
		parse[groups.Interval](t, ca.call("POST", memberBase, map[string]any{"groupId": g2.ID, "from": "2026-05-01"}), 201)
		_, e = pool.Exec(ctx, `SELECT 1 FROM pride.audit_events WHERE entity_type IN ('group_membership','training_group') AND (after_state::text LIKE '%Синтетический%' OR after_state::text LIKE '%375291234567%')`)
		if e != nil {
			t.Fatal(e)
		}
		pool.QueryRow(ctx, `SELECT count(*) FROM pride.audit_events WHERE entity_type IN ('group_membership','training_group') AND (after_state::text LIKE '%Синтетический%' OR after_state::text LIKE '%375291234567%')`).Scan(&n)
		if n != 0 {
			t.Fatal("PII audit")
		}
	})

	t.Run("role revoked after middleware blocks write", func(t *testing.T) {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, `SELECT id FROM pride.staff_users WHERE login='admin-a' FOR UPDATE`); e != nil {
			t.Fatal(e)
		}
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() { result <- ca.call("POST", ga, map[string]any{"name": "revoked"}) }()
		waiting := false
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if e = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT is_owner FROM pride.staff_users%')`).Scan(&waiting); e != nil {
				t.Fatal(e)
			}
			if waiting {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !waiting {
			t.Fatal("write did not reach transaction")
		}
		if _, e = tx.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='admin-a')`); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		select {
		case w := <-result:
			status(t, w, 404)
		case <-time.After(5 * time.Second):
			t.Fatal("revoke deadlock")
		}
		if _, e = pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) SELECT id,$1,'administrator' FROM pride.staff_users WHERE login='admin-a'`, a.ID); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("group stale versions archive club revoke and disable", func(t *testing.T) {
		old := g2
		g2 = parse[groups.Group](t, ca.call("PUT", ga+"/"+g2.ID, map[string]any{"name": "Новое название", "version": g2.Version}), 200)
		status(t, ca.call("PUT", ga+"/"+g2.ID, map[string]any{"name": "stale", "version": old.Version}), 409)
		pool.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='admin-a') AND role='administrator'`)
		status(t, ca.call("GET", ga, nil), 404)
		status(t, ca.call("POST", ga, map[string]any{"name": "new"}), 404)
		pool.Exec(ctx, `UPDATE pride.staff_users SET disabled_at=now() WHERE login='admin-b'`)
		status(t, cb.call("GET", gb, nil), 401)
		parse[clubs.Club](t, owner.call("POST", "/api/clubs/"+a.ID+"/archive", map[string]int{"version": a.Version}), 200)
		status(t, owner.call("POST", ga, map[string]any{"name": "archived"}), 409)
		status(t, owner.call("GET", ga+"?state=all", nil), 200)
	})
}
