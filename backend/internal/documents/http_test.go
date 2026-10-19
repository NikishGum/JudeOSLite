package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/NikishGum/JudeOSLite/backend/internal/participants"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const password = "synthetic-parent-password"
const origin = "https://documents.example.test"

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
	name := fmt.Sprintf("rd11_%d", time.Now().UnixNano())
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
	f.handler = access.Handler(New(pool, access).Handler(families.New(pool, access).Handler(participants.New(pool, access).Handler(clubs.New(pool, access).Handler(fallback)))))
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

func endpoint(club, member string) string {
	return "/api/clubs/" + club + "/participants/" + member + "/documents"
}
func documentJSON(t *testing.T, w *httptest.ResponseRecorder) MemberDocument {
	t.Helper()
	var d MemberDocument
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	return d
}
func (f *fixture) upload(c client, club, member string, data []byte, name string) MemberDocument {
	f.t.Helper()
	w := f.call(c, "POST", endpoint(club, member), input(data, name))
	f.status(w, 201)
	return documentJSON(f.t, w)
}
func (f *fixture) parent(club, member, login string) (client, string, string) {
	f.t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		f.t.Fatal(err)
	}
	user := f.id(`INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,'Synthetic Parent',$2) RETURNING id::text`, login, hash)
	family := f.id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Synthetic family') RETURNING id::text`, club)
	f.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, family, club, member)
	f.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, family, user)
	return f.login(login, password), user, family
}
func (f *fixture) scalar(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}
func TestDocumentLifecycleAndIsolation(t *testing.T) {
	f := setup(t)
	a, b, foreign := f.member(f.a, "Child A"), f.member(f.a, "Child B"), f.member(f.b, "Foreign child")
	pa, _, fa := f.parent(f.a, a, "parent-a")
	pb, _, _ := f.parent(f.a, b, "parent-b")
	for i, tc := range []struct {
		data       []byte
		name, mime string
	}{{pdf, "справка 2026.pdf", "application/pdf"}, {png, "image.png", "image/png"}, {jpeg, "photo.jpg", "image/jpeg"}} {
		payload := input(tc.data, tc.name)
		if i == 0 {
			payload.ExpiresOn = json.RawMessage(`"2028-02-29"`)
		}
		w := f.call(f.admin, "POST", endpoint(f.a, a), payload)
		f.status(w, 201)
		d := documentJSON(t, w)
		if d.ID == "" || d.MembershipID != a || d.Version != 1 || d.MediaType != tc.mime || d.Size != len(tc.data) {
			t.Fatal(d)
		}
		if i == 0 {
			if d.ExpiresOn == nil || *d.ExpiresOn != "2028-02-29" {
				t.Fatal("expiry date lost")
			}
		} else if d.ExpiresOn != nil {
			t.Fatal("unknown expiry changed")
		}
		url := endpoint(f.a, a) + "/" + d.ID + "/download"
		for _, c := range []client{f.owner, f.admin, pa} {
			w := f.call(c, "GET", url, nil)
			f.status(w, 200)
			if !bytes.Equal(w.Body.Bytes(), tc.data) || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Type") != tc.mime {
				t.Fatal("binary changed/unsafe headers")
			}
			disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			if err != nil || disposition != "attachment" || params["filename"] != tc.name {
				t.Fatalf("unsafe attachment %v", err)
			}
			if i == 0 && !strings.Contains(w.Header().Get("Content-Disposition"), "filename*=") {
				t.Fatal("missing Unicode filename*")
			}
		}
		for _, c := range []client{pb, f.trainer} {
			f.status(f.call(c, "GET", url, nil), 404)
		}
		f.status(f.call(client{}, "GET", url, nil), 401)
		f.status(f.call(f.owner, "GET", endpoint(f.a, b)+"/"+d.ID+"/download", nil), 404)
		f.status(f.call(f.owner, "GET", endpoint(f.b, foreign)+"/"+d.ID+"/download", nil), 404)
		f.status(f.call(f.owner, "GET", endpoint(f.b, a)+"/"+d.ID+"/download", nil), 404)
		f.status(f.call(pa, "POST", endpoint(f.a, a), input(tc.data, tc.name)), 404)
		f.status(f.call(f.trainer, "POST", endpoint(f.a, a), input(tc.data, tc.name)), 404)
		f.status(f.call(pa, "POST", endpoint(f.a, a)+"/"+d.ID+"/delete", map[string]int{"version": 1}), 404)
	}
	w := f.call(pa, "GET", endpoint(f.a, a)+"?page=0&limit=2", nil)
	f.status(w, 200)
	var list documentList
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 2 || list.Total != 3 || !list.HasMore || strings.Contains(w.Body.String(), "content") {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(pa, "GET", endpoint(f.a, a)+"?page=1&limit=2", nil), 200)
	f.status(f.call(pb, "GET", endpoint(f.a, a), nil), 404)
	f.status(f.call(f.admin, "GET", endpoint(f.b, foreign), nil), 404)
	for _, q := range []string{"?limit=0", "?page=-1", "?page=1&page=2", "?limit=101"} {
		f.status(f.call(f.owner, "GET", endpoint(f.a, a)+q, nil), 400)
	}
	w = f.call(pa, "GET", endpoint(f.a, a)+"?q=not-found", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal(w.Body.String())
	}
	d := list.Items[0]
	del := endpoint(f.a, a) + "/" + d.ID + "/delete"
	f.status(f.call(f.admin, "POST", del, map[string]int{"version": 2}), 409)
	f.status(f.call(f.admin, "POST", del, map[string]int{}), 400)
	bad := f.admin
	bad.csrf = "invalid"
	f.status(f.call(bad, "POST", del, map[string]int{"version": 1}), 403)
	f.status(f.call(f.admin, "POST", del, map[string]int{"version": 1}), 200)
	f.status(f.call(pa, "GET", endpoint(f.a, a)+"/"+d.ID+"/download", nil), 404)
	f.status(f.call(f.admin, "POST", del, map[string]int{"version": 1}), 404)
	if f.scalar(`SELECT count(*) FROM pride.member_documents`) != 2 || f.scalar(`SELECT count(*) FROM pride.audit_events WHERE entity_type='member_document'`) != 4 {
		t.Fatal("storage/audit lifecycle")
	}
	var payload string
	if err := f.pool.QueryRow(context.Background(), `SELECT jsonb_agg(after_state)::text FROM pride.audit_events WHERE entity_type='member_document'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"medical", "Synthetic medical title", "справка", "expiresOn", "filename", "contentBase64"} {
		if strings.Contains(payload, secret) {
			t.Fatal("private audit payload")
		}
	}
	f.exec(`DELETE FROM pride.family_children WHERE family_id=$1 AND membership_id=$2`, fa, a)
	f.status(f.call(pa, "GET", endpoint(f.a, a), nil), 404)
	f.status(f.call(pa, "GET", endpoint(f.a, a)+"/"+list.Items[1].ID+"/download", nil), 404)
}
func TestDocumentQuotaConcurrencyAndRollback(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Quota Child")
	for i := 0; i < MaxDocuments-1; i++ {
		f.upload(f.owner, f.a, m, pdf, "sample.pdf")
	}
	statuses := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- f.call(f.admin, "POST", endpoint(f.a, m), input(pdf, "last.pdf")).Code
		}()
	}
	wg.Wait()
	close(statuses)
	success := 0
	for status := range statuses {
		if status == 201 {
			success++
		} else if status != 409 {
			t.Fatalf("concurrent upload %d", status)
		}
	}
	if success != 1 || f.scalar(`SELECT count(*) FROM pride.member_documents WHERE membership_id=$1`, m) != 20 {
		t.Fatal("quota race")
	}
	row := f.id(`SELECT id::text FROM pride.member_documents WHERE membership_id=$1 LIMIT 1`, m)
	f.status(f.call(f.owner, "POST", endpoint(f.a, m)+"/"+row+"/delete", map[string]int{"version": 1}), 200)
	f.upload(f.owner, f.a, m, pdf, "freed.pdf")
	n := f.member(f.a, "Volume Child")
	large := append(append([]byte{}, pdf...), bytes.Repeat([]byte{'x'}, MaxFileBytes-len(pdf))...)
	for i := 0; i < 5; i++ {
		f.upload(f.owner, f.a, n, large, "large.pdf")
	}
	f.status(f.call(f.owner, "POST", endpoint(f.a, n), input(pdf, "excess.pdf")), 409)
	f.status(f.call(f.owner, "POST", endpoint(f.a, n), input(append(large, 'x'), "large.pdf")), 413)
	// Failure of audit rolls back both content creation and physical deletion.
	f.exec(`CREATE FUNCTION pride.fail_document_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='member_document' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_document_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.fail_document_audit()`)
	fresh := f.member(f.a, "Rollback Child")
	f.status(f.call(f.owner, "POST", endpoint(f.a, fresh), input(pdf, "rollback.pdf")), 503)
	if f.scalar(`SELECT count(*) FROM pride.member_documents WHERE membership_id=$1`, fresh) != 0 {
		t.Fatal("partial upload")
	}
	row = f.id(`SELECT id::text FROM pride.member_documents WHERE membership_id=$1 LIMIT 1`, m)
	f.status(f.call(f.owner, "POST", endpoint(f.a, m)+"/"+row+"/delete", map[string]int{"version": 1}), 503)
	f.status(f.call(f.owner, "GET", endpoint(f.a, m)+"/"+row+"/download", nil), 200)
	f.exec(`DROP TRIGGER fail_document_audit ON pride.audit_events`)
	statuses = make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- f.call(f.owner, "POST", endpoint(f.a, m)+"/"+row+"/delete", map[string]int{"version": 1}).Code
		}()
	}
	wg.Wait()
	close(statuses)
	a, b := <-statuses, <-statuses
	if !(a == 200 && b == 404 || a == 404 && b == 200) {
		t.Fatalf("delete race %d/%d", a, b)
	}
}
func TestLiveDocumentAuthorization(t *testing.T) {
	for _, change := range []string{"unlink", "revoke", "session", "disable", "administrator"} {
		t.Run(change, func(t *testing.T) {
			f := setup(t)
			m := f.member(f.a, "Live Child")
			d := f.upload(f.owner, f.a, m, pdf, "private.pdf")
			parent, user, family := f.parent(f.a, m, "live-parent")
			actor := parent
			if change == "administrator" {
				actor = f.admin
			}
			access, err := auth.New(f.pool, auth.Config{Origin: origin})
			if err != nil {
				t.Fatal(err)
			}
			svc := New(f.pool, access)
			// Mutate after Require has authenticated the request, before domain SQL.
			f.handler = access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch change {
				case "unlink":
					f.exec(`DELETE FROM pride.family_children WHERE family_id=$1`, family)
				case "revoke":
					f.exec(`DELETE FROM pride.family_access WHERE family_id=$1`, family)
				case "session":
					f.exec(`UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=$1`, user)
				case "disable":
					f.exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, user)
				case "administrator":
					f.exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.adminID)
				}
				r.SetPathValue("clubID", f.a)
				r.SetPathValue("membershipID", m)
				r.SetPathValue("documentID", d.ID)
				w.Header().Set("Cache-Control", "no-store")
				svc.download(w, r)
			}))
			f.status(f.call(actor, "GET", "/test-live", nil), 404)
		})
	}
}
func TestDocumentArchivedAndExpiredSessions(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Archived Child")
	d := f.upload(f.owner, f.a, m, pdf, "archive.pdf")
	pa, user, _ := f.parent(f.a, m, "parent")
	base := endpoint(f.a, m)
	download := base + "/" + d.ID + "/download"
	f.exec(`UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, m)
	f.status(f.call(pa, "GET", download, nil), 200)
	f.exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.a)
	f.status(f.call(pa, "GET", download, nil), 404)
	f.status(f.call(f.admin, "GET", download, nil), 200)
	f.status(f.call(f.admin, "POST", base, input(pdf, "archive.pdf")), 409)
	f.status(f.call(f.admin, "POST", base+"/"+d.ID+"/delete", map[string]int{"version": 1}), 409)
	f.exec(`UPDATE pride.staff_sessions SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 second' WHERE staff_user_id=$1`, user)
	f.status(f.call(pa, "GET", download, nil), 401)
	f.exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, f.adminID)
	f.status(f.call(f.admin, "GET", download, nil), 401)
}

func TestDocumentHTTPInputBoundaries(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Boundary Child")
	base := endpoint(f.a, m)
	for _, tc := range []struct {
		body, ct string
		status   int
	}{
		{`{"name":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, "application/json", 413},
		{`{}`, "text/plain", 400},
		{`{"name":"x",`, "application/json", 400},
		{`{"contentBase64":"%%%"}`, "application/json", 400},
	} {
		r := httptest.NewRequest("POST", base, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.ct)
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", f.owner.csrf)
		for _, cookie := range f.owner.cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		f.status(w, tc.status)
	}
	if f.scalar(`SELECT count(*) FROM pride.member_documents`) != 0 {
		t.Fatal("invalid input stored")
	}
	d := f.upload(f.owner, f.a, m, pdf, "head.pdf")
	w := f.call(f.owner, "HEAD", base+"/"+d.ID+"/download", nil)
	f.status(w, 200)
	if w.Body.Len() != 0 || w.Header().Get("Content-Length") != fmt.Sprint(len(pdf)) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("HEAD binary headers/body")
	}
}
