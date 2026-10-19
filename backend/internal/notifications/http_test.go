package notifications

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/billing"
	"github.com/NikishGum/JudeOSLite/backend/internal/documents"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-inbox-password"
const origin = "https://inbox.example.test"

type client struct {
	cookies []*http.Cookie
	csrf    string
}
type fixture struct {
	t                                                                     *testing.T
	pool                                                                  *pgxpool.Pool
	access                                                                *auth.Service
	s                                                                     *Service
	handler                                                               http.Handler
	owner, admin, trainer, parentA, parentB                               client
	a, b, ownerID, adminID, trainerID, substituteID, parentAID, parentBID string
	familyA, familyB, memberA, memberB, group                             string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("rd12_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
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
	ids := []*string{&f.ownerID, &f.adminID, &f.trainerID, &f.substituteID, &f.parentAID, &f.parentBID}
	for i, login := range []string{"owner", "admin", "trainer", "substitute", "parenta", "parentb"} {
		*ids[i] = f.id(`INSERT INTO pride.staff_users(login,display_name,password_hash,is_owner) VALUES($1,$1,$2,$3) RETURNING id::text`, login, hash, i == 0)
	}
	f.a = f.id(`INSERT INTO pride.clubs(name) VALUES('A') RETURNING id::text`)
	f.b = f.id(`INSERT INTO pride.clubs(name) VALUES('B') RETURNING id::text`)
	f.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$4,'administrator'),($2,$4,'trainer'),($3,$4,'trainer')`, f.adminID, f.trainerID, f.substituteID, f.a)
	for i, target := range []*string{&f.memberA, &f.memberB} {
		p := f.id(`INSERT INTO pride.participants(full_name,birth_date) VALUES($1,'2015-01-01') RETURNING id::text`, fmt.Sprintf("Private child %d", i))
		*target = f.id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2020-01-01','phone','private contact') RETURNING id::text`, f.a, p)
	}
	f.familyA = f.id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Private family A') RETURNING id::text`, f.a)
	f.familyB = f.id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Private family B') RETURNING id::text`, f.a)
	f.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2),($3,$4)`, f.familyA, f.parentAID, f.familyB, f.parentBID)
	f.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3),($4,$2,$5)`, f.familyA, f.a, f.memberA, f.familyB, f.memberB)
	f.group = f.id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Group') RETURNING id::text`, f.a)
	f.access, err = auth.New(pool, auth.Config{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	f.s = New(pool, f.access)
	f.handler = f.access.Handler(f.s.Handler(families.New(pool, f.access).Handler(billing.New(pool, f.access).Handler(documents.New(pool, f.access).Handler(sessions.New(pool, f.access).Handler(http.NotFoundHandler()))))))
	f.owner = f.login("owner")
	f.admin = f.login("admin")
	f.trainer = f.login("trainer")
	f.parentA = f.login("parenta")
	f.parentB = f.login("parentb")
	return f
}
func (f *fixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), q, args...); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) id(q string, args ...any) string {
	f.t.Helper()
	var v string
	if err := f.pool.QueryRow(context.Background(), q, args...).Scan(&v); err != nil {
		f.t.Fatal(err)
	}
	return v
}
func (f *fixture) call(c client, method, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.RemoteAddr = "192.0.2.4:1234"
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
		f.t.Fatal("personal response cacheable")
	}
}
func (f *fixture) login(login string) client {
	w := f.call(client{}, "POST", "/api/auth/login", map[string]any{"login": login, "password": password})
	f.status(w, 200)
	c := client{cookies: w.Result().Cookies()}
	w = f.call(c, "GET", "/api/auth/me", nil)
	f.status(w, 200)
	var me struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		f.t.Fatal(err)
	}
	c.csrf = me.CSRF
	return c
}
func (f *fixture) inbox(c client, query string) Inbox {
	f.t.Helper()
	w := f.call(c, "GET", "/api/notifications"+query, nil)
	f.status(w, 200)
	var result Inbox
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		f.t.Fatal(err)
	}
	if result.Items == nil {
		f.t.Fatal("null items")
	}
	for _, n := range result.Items {
		u, err := url.Parse(n.Href)
		if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(n.Href, "/") || strings.HasPrefix(n.Href, "//") || strings.ContainsAny(n.Href, "\\\r\n") {
			f.t.Fatalf("unsafe href: %q", n.Href)
		}
	}
	return result
}
func (f *fixture) event(family, action string, version int) string {
	return f.id(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,$4,jsonb_build_object('version',$5::int)) RETURNING id::text`, f.a, f.ownerID, family, action, version)
}
func (f *fixture) payment(member string) map[string]any {
	key := f.id(`SELECT gen_random_uuid()::text`)
	body := map[string]any{"requestId": key, "membershipId": member, "description": "Private payment description", "amountMinor": 12345, "method": nil, "paidOn": nil, "planId": nil, "validFrom": nil}
	w := f.call(f.admin, "POST", "/api/clubs/"+f.a+"/payments", body)
	f.status(w, 201)
	var payment map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payment)
	// A lost-response retry must not deliver another event.
	f.status(f.call(f.admin, "POST", "/api/clubs/"+f.a+"/payments", body), 201)
	return payment
}

func TestInboxPersonalPaginationAndRead(t *testing.T) {
	f := setup(t)
	for i := 1; i <= 31; i++ {
		f.event(f.familyA, "family_updated", i)
	}
	f.event(f.familyB, "family_updated", 1)
	// Tied timestamps still have deterministic ID ordering.
	f.exec(`UPDATE pride.notifications SET created_at='2026-01-01T00:00:00Z'`)
	a := f.inbox(f.parentA, "?limit=25")
	if a.Total != 31 || a.UnreadCount != 31 || !a.HasMore || len(a.Items) != 25 {
		t.Fatalf("bad page: %+v", a)
	}
	b := f.inbox(f.parentA, "?page=1&limit=25")
	if b.Total != 31 || b.HasMore || len(b.Items) != 6 {
		t.Fatalf("bad last page: %+v", b)
	}
	seen := map[string]bool{}
	for _, item := range append(a.Items, b.Items...) {
		if seen[item.ID] {
			t.Fatal("duplicate page item")
		}
		seen[item.ID] = true
	}
	if f.inbox(f.parentA, "?page=100&limit=25").Total != 31 {
		t.Fatal("empty page lost totals")
	}
	if f.inbox(f.parentB, "").Total != 1 || f.inbox(f.trainer, "").Total != 0 {
		t.Fatal("foreign inbox leaked")
	}
	foreign := f.inbox(f.parentB, "").Items[0].ID
	f.status(f.call(f.parentA, "POST", "/api/notifications/"+foreign+"/read", map[string]any{}), 404)
	f.status(f.call(f.owner, "POST", "/api/notifications/"+a.Items[0].ID+"/read", map[string]any{}), 404)
	path := "/api/notifications/" + a.Items[0].ID + "/read"
	for i := 0; i < 2; i++ {
		f.status(f.call(f.parentA, "POST", path, map[string]any{}), 200)
	}
	if got := f.inbox(f.parentA, "?status=unread"); got.Total != 30 || got.UnreadCount != 30 {
		t.Fatalf("read mismatch %+v", got)
	}
	first := f.id(`SELECT read_at::text FROM pride.notifications WHERE id=$1`, a.Items[0].ID)
	for i := 0; i < 2; i++ {
		f.status(f.call(f.parentA, "POST", "/api/notifications/read-all", map[string]any{}), 200)
	}
	if f.id(`SELECT read_at::text FROM pride.notifications WHERE id=$1`, a.Items[0].ID) != first {
		t.Fatal("read time changed")
	}
	if got := f.inbox(f.parentA, "?status=unread"); got.Total != 0 || got.UnreadCount != 0 || got.HasMore {
		t.Fatal("read-all incomplete")
	}
	if f.inbox(f.parentB, "").UnreadCount != 1 {
		t.Fatal("read-all changed another recipient")
	}
	for _, q := range []string{"?page=-1", "?page=1000001", "?page=x", "?limit=0", "?limit=101", "?status=other", "?status=", "?page=0&page=1", "?userId=x"} {
		f.status(f.call(f.parentA, "GET", "/api/notifications"+q, nil), 400)
	}
	for _, body := range []any{nil, []any{}, map[string]any{"id": foreign}} {
		f.status(f.call(f.parentA, "POST", path, body), 400)
	}
	f.status(f.call(f.parentA, "POST", "/api/notifications/not-a-uuid/read", map[string]any{}), 404)
	csrf := f.parentA
	csrf.csrf = ""
	f.status(f.call(csrf, "POST", path, map[string]any{}), 403)
	f.status(f.call(client{}, "GET", "/api/notifications", nil), 401)
}

func TestDomainEventsPrivacyAndRevocation(t *testing.T) {
	f := setup(t)
	p := f.payment(f.memberA)
	base := "/api/clubs/" + f.a + "/payments/" + p["id"].(string)
	f.status(f.call(f.admin, "POST", base+"/confirm", map[string]any{"version": 1, "method": "cash", "paidOn": "2026-10-19"}), 200)
	f.status(f.call(f.admin, "POST", base+"/void", map[string]any{"version": 2, "reason": "Private reason"}), 200)
	docbase := "/api/clubs/" + f.a + "/participants/" + f.memberA + "/documents"
	w := f.call(f.admin, "POST", docbase, map[string]any{"name": "Private medical title", "kind": "medical", "filename": "private.pdf", "contentBase64": base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\n%%EOF")), "expiresOn": nil})
	f.status(w, 201)
	var doc struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &doc)
	f.status(f.call(f.admin, "POST", docbase+"/"+doc.ID+"/delete", map[string]any{"version": 1}), 200)
	a := f.inbox(f.parentA, "")
	if a.Total != 5 || a.UnreadCount != 5 {
		t.Fatalf("missing domain events %+v", a)
	}
	if f.inbox(f.parentB, "").Total != 0 || f.inbox(f.trainer, "").Total != 0 {
		t.Fatal("private domain leak")
	}
	if f.inbox(f.owner, "").Total != 5 || f.inbox(f.admin, "").Total != 5 {
		t.Fatal("manager events missing")
	}
	raw, _ := json.Marshal(a)
	for _, private := range []string{"Private", "medical", "private.pdf", "12345", "contact"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("PII in notification: %s", raw)
		}
	}
	for _, n := range a.Items {
		if strings.HasPrefix(n.Href, "/clubs/") {
			t.Fatal("parent got staff href")
		}
	}
	// Unlink makes child events unavailable, including direct read and bulk read.
	f.exec(`DELETE FROM pride.family_children WHERE membership_id=$1`, f.memberA)
	if f.inbox(f.parentA, "").Total != 0 {
		t.Fatal("unlink preserved child events")
	}
	f.status(f.call(f.parentA, "POST", "/api/notifications/"+a.Items[0].ID+"/read", map[string]any{}), 404)
	f.status(f.call(f.parentA, "POST", "/api/notifications/read-all", map[string]any{}), 200)
	if f.id(`SELECT count(*)::text FROM pride.notifications WHERE user_id=$1 AND read_at IS NOT NULL`, f.parentAID) != "0" {
		t.Fatal("bulk read changed hidden messages")
	}
	// Moving the child to B does not expose A's past messages to either family.
	f.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, f.familyB, f.a, f.memberA)
	if f.inbox(f.parentA, "").Total != 0 || f.inbox(f.parentB, "").Total != 0 {
		t.Fatal("transfer exposed past messages")
	}
	f.event(f.familyA, "family_updated", 2)
	f.exec(`DELETE FROM pride.family_access WHERE user_id=$1`, f.parentAID)
	if f.inbox(f.parentA, "").Total != 0 {
		t.Fatal("family revoke retained inbox")
	}
	f.exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.adminID)
	if f.inbox(f.admin, "").UnreadCount != 0 {
		t.Fatal("admin revoke retained counter")
	}
	// A foreign club event goes to owner only, not this club's administrator.
	family := f.id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Other') RETURNING id::text`, f.b)
	f.exec(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,'family_created','{"version":1}')`, f.b, f.ownerID, family)
	if f.inbox(f.parentB, "").Total != 0 {
		t.Fatal("foreign club event leaked")
	}
}

func TestFamilyAccountEventsAndLiveSession(t *testing.T) {
	f := setup(t)
	path := "/api/clubs/" + f.a + "/families/" + f.familyA + "/accounts"
	w := f.call(f.admin, "POST", path, map[string]any{"login": "newparent", "displayName": "Private parent", "password": password, "version": 1})
	f.status(w, 201)
	var family struct {
		Accounts []struct {
			ID string `json:"id"`
		} `json:"accounts"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &family)
	newID := f.id(`SELECT id::text FROM pride.staff_users WHERE login='newparent'`)
	parent := f.login("newparent")
	if f.inbox(parent, "").Total != 1 {
		t.Fatal("new account not notified")
	}
	f.status(f.call(f.admin, "POST", path+"/"+newID+"/revoke", map[string]any{"version": 2}), 200)
	if f.inbox(parent, "").Total != 0 {
		t.Fatal("revoked new parent sees account event")
	}
	// Revoke after Require but before the inbox transaction: never rely on the
	// middleware's cached user/session/assignments for personal data.
	tests := []struct {
		name     string
		c        client
		mutation string
		args     []any
		want     int
	}{
		{"family", f.parentA, `DELETE FROM pride.family_access WHERE user_id=$1`, []any{f.parentAID}, 200},
		{"role", f.admin, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, []any{f.adminID}, 200},
		{"session", f.trainer, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1`, []any{f.trainerID}, 404},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f.handler = f.access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store") // normally set by the outer Handler
				f.exec(tc.mutation, tc.args...)
				f.s.list(w, r)
			}))
			w := f.call(tc.c, "GET", "/api/notifications", nil)
			f.status(w, tc.want)
			if tc.want == 200 {
				var inbox Inbox
				_ = json.Unmarshal(w.Body.Bytes(), &inbox)
				if inbox.Total != 0 || inbox.UnreadCount != 0 {
					t.Fatal("live revoke leaked messages")
				}
			}
		})
	}
}

func TestTrainingEventsAndGenerator(t *testing.T) {
	f := setup(t)
	path := "/api/clubs/" + f.a + "/sessions"
	w := f.call(f.admin, "POST", path, map[string]any{"groupId": f.group, "trainerId": f.trainerID, "date": "2030-02-10", "startsTime": "10:00", "durationMinutes": 60})
	f.status(w, 201)
	var session struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &session)
	if got := f.inbox(f.trainer, ""); got.Total != 1 || !strings.Contains(got.Items[0].Href, "session="+session.ID) {
		t.Fatal("assigned trainer not notified")
	}
	f.status(f.call(f.admin, "POST", path+"/"+session.ID+"/reschedule", map[string]any{"version": 1, "date": "2030-02-11", "startsTime": "11:00", "durationMinutes": 60}), 200)
	if f.inbox(f.trainer, "").Total != 2 {
		t.Fatal("move event missing")
	}
	f.status(f.call(f.admin, "POST", path+"/"+session.ID+"/replace-trainer", map[string]any{"version": 2, "trainerId": f.substituteID}), 200)
	if f.inbox(f.trainer, "").Total != 0 {
		t.Fatal("former trainer sees old notifications")
	}
	sub := f.login("substitute")
	if f.inbox(sub, "").Total != 1 {
		t.Fatal("replacement not notified")
	}
	f.status(f.call(f.admin, "POST", path+"/"+session.ID+"/cancel", map[string]any{"version": 3, "reason": "Private reason"}), 200)
	if f.inbox(sub, "").Total != 2 || f.inbox(f.parentA, "").Total != 0 {
		t.Fatal("cancel recipient scope incorrect")
	}
	// Generator INSERTs have no human audit event. Both first generation and
	// repeating it must still produce exactly one inbox row per recipient/event.
	f.exec(`INSERT INTO pride.schedule_rules(club_id,group_id,trainer_id,mode,weekdays,starts_time,duration_minutes,timezone,valid_from,valid_until) VALUES($1,$2,$3,'weekly',ARRAY[1,2,3,4,5,6,7],'15:00',60,'Europe/Minsk','2030-02-12','2030-02-13')`, f.a, f.group, f.trainerID)
	s := schedule.New(f.pool, f.access)
	for i := 0; i < 2; i++ {
		if err := s.Generate(context.Background(), time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	if f.inbox(f.trainer, "").Total != 1 {
		t.Fatal("generator missing or duplicate event")
	}
	f.exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.trainerID)
	if f.inbox(f.trainer, "").UnreadCount != 0 {
		t.Fatal("trainer role revoke retained inbox")
	}
}

func TestTransactionalRollbackReplayAndConcurrentRead(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,'family_updated','{"version":10}')`, f.a, f.ownerID, f.familyA)
	if err != nil {
		t.Fatal(err)
	}
	if f.inbox(f.parentA, "").Total != 0 {
		t.Fatal("uncommitted event visible")
	}
	_ = tx.Rollback(ctx)
	if f.inbox(f.parentA, "").Total != 0 {
		t.Fatal("rollback retained event")
	}
	// Roll back a training insert too, including its row-trigger delivery.
	tx, err = f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,'2030-02-10','Europe/Minsk','2030-02-10T10:00Z','2030-02-10T11:00Z')`, f.a, f.group, f.trainerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if f.inbox(f.trainer, "").Total != 0 {
		t.Fatal("rolled back training event visible")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.pool.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,'family_updated','{"version":11}')`, f.a, f.ownerID, f.familyA)
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	got := f.inbox(f.parentA, "")
	if got.Total != 1 {
		t.Fatal("replayed event duplicated")
	}
	statuses := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- f.call(f.parentA, "POST", "/api/notifications/"+got.Items[0].ID+"/read", map[string]any{}).Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concurrent read: %d", status)
		}
	}
	if f.inbox(f.parentA, "").UnreadCount != 0 {
		t.Fatal("concurrent read not persisted")
	}
	// A failed notification insert must abort the source HTTP mutation and audit.
	f.exec(`CREATE FUNCTION pride.reject_notification() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic inbox failure'; END $$; CREATE TRIGGER reject_notification BEFORE INSERT ON pride.notifications FOR EACH ROW EXECUTE FUNCTION pride.reject_notification()`)
	key := f.id(`SELECT gen_random_uuid()::text`)
	f.status(f.call(f.admin, "POST", "/api/clubs/"+f.a+"/payments", map[string]any{"requestId": key, "membershipId": f.memberA, "description": "Rollback", "amountMinor": 1, "method": nil, "paidOn": nil, "planId": nil, "validFrom": nil}), 503)
	if f.id(`SELECT count(*)::text FROM pride.payments`) != "0" || f.id(`SELECT count(*)::text FROM pride.audit_events WHERE entity_type='payment'`) != "0" {
		t.Fatal("notification failure partially committed payment")
	}
}

func TestMixedRolesArchiveDisableAndHTTPEnvelope(t *testing.T) {
	f := setup(t)
	// A parent+trainer retains the independent family permission but cannot read
	// other families merely because the club is discoverable through training.
	f.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, f.parentAID, f.a)
	f.event(f.familyA, "family_updated", 1)
	f.event(f.familyB, "family_updated", 1)
	if f.inbox(f.parentA, "").Total != 1 {
		t.Fatal("mixed roles broadened family access")
	}
	f.exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.a)
	if f.inbox(f.parentA, "").UnreadCount != 0 {
		t.Fatal("archived club exposed parent inbox")
	}
	if f.inbox(f.admin, "").Total != 2 {
		t.Fatal("manager lost permitted archive history")
	}
	f.exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, f.adminID)
	f.status(f.call(f.admin, "GET", "/api/notifications", nil), 401)
	f.exec(`UPDATE pride.staff_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE staff_user_id=$1`, f.parentBID)
	f.status(f.call(f.parentB, "GET", "/api/notifications", nil), 401)
	// Wrong method remains no-store, including an unsupported collection method.
	f.status(f.call(f.owner, "DELETE", "/api/notifications", nil), 404)
	r := httptest.NewRequest("POST", "/api/notifications/read-all", strings.NewReader("{}"))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "text/plain")
	r.Header.Set("X-CSRF-Token", f.owner.csrf)
	for _, c := range f.owner.cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	f.status(w, 400)
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
}
