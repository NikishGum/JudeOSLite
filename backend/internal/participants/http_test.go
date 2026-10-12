package participants

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
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-participants-password"
const origin = "https://participants.example.test"

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
	name := fmt.Sprintf("judeos_participants_%d", time.Now().UnixNano())
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

func input(c Card, identity bool) cardInput {
	in := cardInput{JoinedOn: c.JoinedOn, ContactKind: c.ContactKind, ContactValue: c.ContactValue, ContactName: c.ContactName, Version: c.Version}
	if identity {
		in.Identity = &IdentityInput{FullName: c.FullName, BirthDate: c.BirthDate, Version: c.ParticipantVersion}
	}
	return in
}
func newInput() cardInput {
	return cardInput{Identity: &IdentityInput{FullName: "Синтетический Участник", BirthDate: "2014-02-28"}, JoinedOn: "2025-09-01", ContactKind: "phone", ContactValue: "+375 (29) 123-45-67", ContactName: "Синтетический Родитель"}
}
func TestRegistryIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	if err := auth.CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", password); err != nil {
		t.Fatal(err)
	}
	access, err := auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	h := access.Handler(New(pool, access).Handler(clubs.New(pool, access).Handler(http.NotFoundHandler())))
	owner := login(t, h, "owner")
	a := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "Synthetic A", "timezone": "Europe/Minsk"}), 201)
	b := parse[clubs.Club](t, owner.call("POST", "/api/clubs", map[string]string{"name": "Synthetic B", "timezone": "UTC"}), 201)
	for _, name := range []string{"admin-a", "admin-b", "trainer"} {
		clubID := a.ID
		if name == "admin-b" {
			clubID = b.ID
		}
		var userID string
		if err := pool.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		role := "administrator"
		if name == "trainer" {
			role = "trainer"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)`, userID, clubID, role); err != nil {
			t.Fatal(err)
		}
	}
	ca, cb, trainer := login(t, h, "admin-a"), login(t, h, "admin-b"), login(t, h, "trainer")
	baseA, baseB := "/api/clubs/"+a.ID+"/participants", "/api/clubs/"+b.ID+"/participants"
	first := parse[Card](t, ca.call("POST", baseA, newInput()), 201)
	duplicate := parse[Card](t, ca.call("POST", baseA, newInput()), 201)
	if first.ID == duplicate.ID || first.ParticipantID == duplicate.ParticipantID {
		t.Fatal("identity matched by name or phone")
	}
	foreignIn := newInput()
	foreignIn.ContactValue = "+375291111111"
	foreignIn.ContactName = "PRIVATE B"
	foreign := parse[Card](t, cb.call("POST", baseB, foreignIn), 201)

	t.Run("scoped reads search and all write boundaries", func(t *testing.T) {
		status(t, client{h: h}.call("GET", baseA, nil), 401)
		for _, method := range []string{"GET", "POST"} {
			status(t, trainer.call(method, baseA, newInput()), 404)
			status(t, cb.call(method, baseA, newInput()), 404)
		}
		for _, path := range []string{baseA + "/" + foreign.ID, baseB + "/" + foreign.ID, baseA + "/not-an-id"} {
			for _, method := range []string{"GET", "PUT"} {
				status(t, ca.call(method, path, input(first, true)), 404)
			}
			for _, action := range []string{"archive", "restore"} {
				status(t, ca.call("POST", path+"/"+action, map[string]int{"version": 1}), 404)
			}
		}
		denied := ca.call("GET", baseB, nil)
		missing := ca.call("GET", "/api/clubs/missing/participants", nil)
		if denied.Body.String() != missing.Body.String() {
			t.Fatal("foreign existence disclosed")
		}
		for _, q := range []string{"Участник", "123-45", "Родитель"} {
			list := parse[struct{ Items []Card }](t, ca.call("GET", baseA+"?q="+q, nil), 200)
			if len(list.Items) != 2 {
				t.Fatal("search lost duplicates", q, len(list.Items))
			}
		}
		response := ca.call("GET", baseA+"?state=all", nil)
		status(t, response, 200)
		if strings.Contains(response.Body.String(), "PRIVATE B") || strings.Contains(response.Body.String(), foreign.ID) {
			t.Fatal("foreign contact leaked")
		}
		status(t, ca.call("GET", baseA+"?q=%25", nil), 200) // literal %, not an SQL wildcard
		noCSRF := ca
		noCSRF.csrf = ""
		for _, path := range []string{baseA, baseA + "/" + first.ID + "/archive", baseA + "/" + first.ID + "/restore"} {
			status(t, noCSRF.call("POST", path, newInput()), 403)
		}
		status(t, noCSRF.call("PUT", baseA+"/"+first.ID, input(first, true)), 403)
		status(t, ca.call("POST", baseA, map[string]any{"participantId": foreign.ParticipantID}), 400)
	})
	t.Run("manual dates and contact validation", func(t *testing.T) {
		for _, birth := range []string{"", "2014", "2014-02-29", "0000-01-01", "9999-01-01", "2025-09-02"} {
			in := newInput()
			in.Identity.BirthDate = birth
			status(t, ca.call("POST", baseA, in), 400)
		}
		for _, joined := range []string{"", "2025-02-30", "2010-01-01", "9999-01-01"} {
			in := newInput()
			in.JoinedOn = joined
			status(t, ca.call("POST", baseA, in), 400)
		}
		for _, phone := range []string{"", "123", "+1234567890123456", "abc1234567", "123+456789", "1234567\n8"} {
			in := newInput()
			in.ContactValue = phone
			status(t, ca.call("POST", baseA, in), 400)
		}
		for _, email := range []string{"", "invalid", "Name <a@example.test>", "a @example.test"} {
			in := newInput()
			in.ContactKind = "email"
			in.ContactValue = email
			status(t, ca.call("POST", baseA, in), 400)
		}
		in := newInput()
		in.Identity.FullName = "   "
		status(t, ca.call("POST", baseA, in), 400)
		in = newInput()
		in.ContactName = "bad\x00name"
		status(t, ca.call("POST", baseA, in), 400)
		in = newInput()
		in.Identity = nil
		status(t, ca.call("POST", baseA, in), 400)
		in = newInput()
		in.Identity.BirthDate = "2016-02-29"
		in.ContactKind = "email"
		in.ContactValue = "family@example.test"
		card := parse[Card](t, ca.call("POST", baseA, in), 201)
		if card.BirthDate != "2016-02-29" {
			t.Fatal("date fabricated")
		}
		in = input(card, true)
		in.JoinedOn = "2016-02-29"
		parse[Card](t, ca.call("PUT", baseA+"/"+card.ID, in), 200)
	})
	t.Run("single identity and concurrent edits", func(t *testing.T) {
		in := input(first, true)
		in.Identity.FullName = "Изменённое Синтетическое Имя"
		first = parse[Card](t, ca.call("PUT", baseA+"/"+first.ID, in), 200)
		if !first.CanEditIdentity || first.ParticipantVersion != 2 {
			t.Fatal("single-club admin cannot edit identity")
		}
		var wg sync.WaitGroup
		results := make(chan *httptest.ResponseRecorder, 2)
		for _, contact := range []string{"+375291234568", "+375291234569"} {
			in := input(first, false)
			in.ContactValue = contact
			wg.Go(func() { results <- ca.call("PUT", baseA+"/"+first.ID, in) })
		}
		wg.Wait()
		close(results)
		ok, conflict := 0, 0
		for w := range results {
			switch w.Code {
			case 200:
				ok++
			case 409:
				conflict++
			default:
				t.Fatal(w.Code, w.Body.String())
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatal("silent overwrite", ok, conflict)
		}
		stale := first
		first = parse[Card](t, ca.call("GET", baseA+"/"+first.ID, nil), 200)
		status(t, ca.call("POST", baseA+"/"+first.ID+"/archive", map[string]int{"version": stale.Version}), 409)
	})
	t.Run("shared identity owner control and isolated contacts", func(t *testing.T) {
		var linked string
		if err := pool.QueryRow(ctx, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value,contact_name) VALUES($1,$2,'2025-09-01','email','private-b@example.test','PRIVATE B') RETURNING id::text`, first.ParticipantID, b.ID).Scan(&linked); err != nil {
			t.Fatal(err)
		}
		first = parse[Card](t, ca.call("GET", baseA+"/"+first.ID, nil), 200)
		sharedB := parse[Card](t, cb.call("GET", baseB+"/"+linked, nil), 200)
		if first.CanEditIdentity || sharedB.CanEditIdentity {
			t.Fatal("shared identity editable by admin")
		}
		in := input(first, true)
		in.Identity.FullName = "Forbidden change"
		status(t, ca.call("PUT", baseA+"/"+first.ID, in), 403)
		in = input(first, false)
		in.ContactName = "Local Contact Only"
		first = parse[Card](t, ca.call("PUT", baseA+"/"+first.ID, in), 200)
		afterB := parse[Card](t, cb.call("GET", baseB+"/"+linked, nil), 200)
		if afterB != sharedB {
			t.Fatal("local change overwrote other club")
		}
		ownerA := parse[Card](t, owner.call("GET", baseA+"/"+first.ID, nil), 200)
		in = input(ownerA, true)
		in.Identity.FullName = "Общее Изменённое Имя"
		first = parse[Card](t, owner.call("PUT", baseA+"/"+first.ID, in), 200)
		afterB = parse[Card](t, cb.call("GET", baseB+"/"+linked, nil), 200)
		if afterB.FullName != first.FullName || afterB.ContactValue != sharedB.ContactValue || afterB.Version != sharedB.Version {
			t.Fatal("shared identity/local version contract broken")
		}
		// The shared identity version, independent of local version, detects an old owner form.
		staleOwner := sharedB
		staleOwner.CanEditIdentity = true
		status(t, owner.call("PUT", baseB+"/"+linked, input(staleOwner, true)), 409)
		in = input(first, true)
		in.Identity.BirthDate = "2025-09-02"
		in.JoinedOn = "2025-09-02"
		status(t, owner.call("PUT", baseA+"/"+first.ID, in), 400) // would violate B's joined_on
		sharedB = parse[Card](t, cb.call("POST", baseB+"/"+linked+"/archive", map[string]int{"version": afterB.Version}), 200)
		localA := parse[Card](t, ca.call("GET", baseA+"/"+first.ID, nil), 200)
		if localA.CanEditIdentity {
			t.Fatal("archived other link discarded for permissions")
		}
		raw := ca.call("GET", baseA+"?state=all", nil)
		if strings.Contains(raw.Body.String(), "private-b@example.test") || strings.Contains(raw.Body.String(), linked) {
			t.Fatal("shared contact/link leaked")
		}
	})
	t.Run("archive return preserves historical composite references", func(t *testing.T) {
		// A test-only historical consumer exercises the key future training tasks use.
		if _, err := pool.Exec(ctx, `CREATE TABLE pride.synthetic_history(membership_id uuid NOT NULL,club_id uuid NOT NULL,FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id) ON DELETE RESTRICT)`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO pride.synthetic_history VALUES($1,$2)`, duplicate.ID, a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO pride.synthetic_history VALUES($1,$2)`, duplicate.ID, b.ID); err == nil {
			t.Fatal("cross-club history FK accepted")
		}
		duplicate = parse[Card](t, ca.call("POST", baseA+"/"+duplicate.ID+"/archive", map[string]int{"version": duplicate.Version}), 200)
		if !duplicate.Archived {
			t.Fatal("not archived")
		}
		status(t, ca.call("GET", baseA+"/"+duplicate.ID, nil), 200)
		list := parse[struct{ Items []Card }](t, ca.call("GET", baseA+"?state=archived", nil), 200)
		if len(list.Items) != 1 || list.Items[0].ID != duplicate.ID {
			t.Fatal("archive filter")
		}
		duplicate = parse[Card](t, ca.call("POST", baseA+"/"+duplicate.ID+"/restore", map[string]int{"version": duplicate.Version}), 200)
		if duplicate.Archived || duplicate.ParticipantVersion != 1 {
			t.Fatal("restore changed identity")
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pride.synthetic_history h JOIN pride.club_memberships m ON m.id=h.membership_id AND m.club_id=h.club_id WHERE m.id=$1`, duplicate.ID).Scan(&count); err != nil || count != 1 {
			t.Fatal("history lost", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM pride.club_memberships WHERE id=$1`, duplicate.ID); err == nil {
			t.Fatal("historical membership deleted")
		}
		if _, err := pool.Exec(ctx, `DELETE FROM pride.participants WHERE id=$1`, duplicate.ParticipantID); err == nil {
			t.Fatal("identity deleted under membership")
		}
		status(t, ca.call("DELETE", baseA+"/"+duplicate.ID, nil), 404)
	})
	t.Run("identity lock serializes link insertion", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SELECT id FROM pride.participants WHERE id=$1 FOR UPDATE`, duplicate.ParticipantID); err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() {
			_, err := pool.Exec(ctx, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2025-09-01','phone','1234567')`, duplicate.ParticipantID, b.ID)
			result <- err
		}()
		select {
		case err := <-result:
			t.Fatal("link crossed identity lock", err)
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("link deadlocked")
		}
		current := parse[Card](t, ca.call("GET", baseA+"/"+duplicate.ID, nil), 200)
		if current.CanEditIdentity {
			t.Fatal("new link not observed")
		}
		status(t, ca.call("PUT", baseA+"/"+duplicate.ID, input(duplicate, true)), 403)
	})
	t.Run("audit rollback and privacy", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `CREATE FUNCTION pride.reject_registry_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic rejection'; END $$; CREATE TRIGGER reject_registry_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_registry_audit()`); err != nil {
			t.Fatal(err)
		}
		status(t, ca.call("POST", baseA, newInput()), 503)
		before := parse[Card](t, owner.call("GET", baseA+"/"+first.ID, nil), 200)
		in := input(before, true)
		in.Identity.FullName = "must rollback"
		in.ContactValue = "7654321"
		status(t, owner.call("PUT", baseA+"/"+first.ID, in), 503)
		status(t, ca.call("POST", baseA+"/"+first.ID+"/archive", map[string]int{"version": before.Version}), 503)
		after := parse[Card](t, owner.call("GET", baseA+"/"+first.ID, nil), 200)
		if before != after {
			t.Fatal("mutation survived audit failure")
		}
		var orphan int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pride.participants p WHERE NOT EXISTS(SELECT 1 FROM pride.club_memberships m WHERE m.participant_id=p.id)`).Scan(&orphan); err != nil || orphan != 0 {
			t.Fatal("create partially committed", err)
		}
		if _, err := pool.Exec(ctx, `DROP TRIGGER reject_registry_audit ON pride.audit_events; DROP FUNCTION pride.reject_registry_audit()`); err != nil {
			t.Fatal(err)
		}
		var payload string
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*),string_agg(coalesce(before_state::text,'')||coalesce(after_state::text,''),'') FROM pride.audit_events WHERE entity_type='club_membership'`).Scan(&count, &payload); err != nil {
			t.Fatal(err)
		}
		if count < 10 {
			t.Fatal("missing audit", count)
		}
		for _, secret := range []string{"Участник", "Изменённое", "2014-02-28", "123-45-67", "private-b@example.test", "Local Contact", "password", "Родитель"} {
			if strings.Contains(payload, secret) {
				t.Fatal("PII in audit", secret)
			}
		}
	})
	t.Run("bounded pagination", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `WITH p AS (INSERT INTO pride.participants(full_name,birth_date) SELECT 'Pagination '||n,'2010-01-01'::date FROM generate_series(1,51) n RETURNING id) INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) SELECT id,$1,'2025-01-01','phone','1234567' FROM p`, a.ID); err != nil {
			t.Fatal(err)
		}
		type listing struct {
			Items   []Card
			HasMore bool
		}
		one := parse[listing](t, ca.call("GET", baseA+"?q=Pagination", nil), 200)
		two := parse[listing](t, ca.call("GET", baseA+"?q=Pagination&page=1", nil), 200)
		if len(one.Items) != 50 || !one.HasMore || len(two.Items) != 1 || two.HasMore {
			t.Fatal("pagination invalid")
		}
		for _, query := range []string{"?page=-1", "?page=bad", "?state=bad"} {
			status(t, ca.call("GET", baseA+query, nil), 400)
		}
	})
	t.Run("write rechecks role after waiting for actor lock", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SELECT id FROM pride.staff_users WHERE login='admin-a' FOR UPDATE`); err != nil {
			t.Fatal(err)
		}
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() { result <- ca.call("POST", baseA, newInput()) }()
		// Confirm the initial HTTP checks have passed and the write is waiting at
		// the transaction's live actor check; then revoke before it can proceed.
		deadline := time.Now().Add(5 * time.Second)
		waiting := false
		for time.Now().Before(deadline) {
			if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT is_owner FROM pride.staff_users%')`).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !waiting {
			t.Fatal("request did not reach transaction check")
		}
		if _, err = tx.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='admin-a')`); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case w := <-result:
			status(t, w, 404)
		case <-time.After(5 * time.Second):
			t.Fatal("revoked write deadlocked")
		}
		if _, err = pool.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) SELECT id,$1,'administrator' FROM pride.staff_users WHERE login='admin-a'`, a.ID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("revocation disable and archived club", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE login='admin-a')`); err != nil {
			t.Fatal(err)
		}
		status(t, ca.call("GET", baseA, nil), 404)
		status(t, ca.call("POST", baseA, newInput()), 404)
		if _, err := pool.Exec(ctx, `UPDATE pride.staff_users SET disabled_at=now() WHERE login='admin-b'`); err != nil {
			t.Fatal(err)
		}
		status(t, cb.call("GET", baseB, nil), 401)
		parse[clubs.Club](t, owner.call("POST", "/api/clubs/"+a.ID+"/archive", map[string]int{"version": a.Version}), 200)
		status(t, owner.call("GET", baseA, nil), 200)
		status(t, owner.call("GET", baseA+"/"+first.ID, nil), 200)
		status(t, owner.call("POST", baseA, newInput()), 409)
		status(t, owner.call("PUT", baseA+"/"+first.ID, input(first, false)), 409)
		for _, action := range []string{"archive", "restore"} {
			status(t, owner.call("POST", baseA+"/"+first.ID+"/"+action, map[string]int{"version": first.Version}), 409)
		}
	})
}
