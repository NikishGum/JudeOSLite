package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func trainerResult(t *testing.T, w *httptest.ResponseRecorder, want int) Trainer {
	t.Helper()
	status(t, w, want)
	var u Trainer
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	return u
}
func TestTrainersIntegration(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	if err := CreateFirstOwner(ctx, pool, "owner", "Synthetic Owner", testPassword); err != nil {
		t.Fatal(err)
	}
	s, err := New(pool, Config{Origin: testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler(http.NotFoundHandler())
	a := queryID(t, pool, `INSERT INTO pride.clubs(name) VALUES('A') RETURNING id::text`)
	b := queryID(t, pool, `INSERT INTO pride.clubs(name) VALUES('B') RETURNING id::text`)
	hash, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	adminA := queryID(t, pool, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('admin-a','Admin A',$1) RETURNING id::text`, hash)
	adminB := queryID(t, pool, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('admin-b','Admin B',$1) RETURNING id::text`, hash)
	execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator'),($3,$4,'administrator')`, adminA, a, adminB, b)
	type actor struct {
		cookies []*http.Cookie
		csrf    string
	}
	owner := actor{}
	owner.cookies, owner.csrf = loginAs(t, h, "owner")
	aa := actor{}
	aa.cookies, aa.csrf = loginAs(t, h, "admin-a")
	bb := actor{}
	bb.cookies, bb.csrf = loginAs(t, h, "admin-b")
	request := func(c actor, method, path string, body any) *httptest.ResponseRecorder {
		return call(h, method, path, testOrigin, c.csrf, c.cookies, body)
	}
	base := "/api/clubs/" + a + "/trainers"
	create := func(name string) Trainer {
		return trainerResult(t, request(aa, "POST", base, map[string]any{"login": name, "displayName": "Synthetic " + name, "password": testPassword}), 201)
	}
	account := func(u Trainer) any {
		return map[string]any{"login": u.Login, "displayName": u.DisplayName, "disabled": u.Disabled, "version": u.Version}
	}
	local := func(u Trainer) string { return base + "/" + u.ID }
	t.Run("creation login boundaries selection", func(t *testing.T) {
		status(t, call(h, "GET", base, "", "", nil, nil), 401)
		u := create("coach-a")
		if !u.CanManageAccount || u.Version != 1 {
			t.Fatal("local account permissions")
		}
		coach := actor{}
		coach.cookies, coach.csrf = loginAs(t, h, u.Login)
		status(t, request(coach, "GET", base, nil), 404)
		status(t, request(bb, "GET", base, nil), 404)
		status(t, request(aa, "POST", base, map[string]any{"login": " COACH-A ", "displayName": "Other", "password": testPassword}), 409)
		for _, field := range []string{"role", "isOwner", "clubIds"} {
			in := map[string]any{"login": "forged", "displayName": "Forged", "password": testPassword, field: true}
			status(t, request(aa, "POST", base, in), 400)
		}
		status(t, call(h, "POST", base, testOrigin, "", aa.cookies, map[string]string{}), 403)
		status(t, request(aa, "GET", "/api/trainer-accounts", nil), 403)
		status(t, request(aa, "POST", "/api/staff/"+u.ID+"/trainer-assignments", map[string]any{"clubId": b, "version": u.Version}), 403)
		list := request(aa, "GET", base+"?available=true", nil)
		status(t, list, 200)
		if !strings.Contains(list.Body.String(), u.ID) || strings.Contains(list.Body.String(), "login") || strings.Contains(list.Body.String(), b) {
			t.Fatal("selection payload")
		}
		u.DisplayName = "Renamed"
		u.Login = "coach-renamed"
		updated := trainerResult(t, request(aa, "PUT", local(u)+"/account", account(u)), 200)
		status(t, request(coach, "GET", "/api/auth/me", nil), 401)
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 409)
		coach.cookies, coach.csrf = loginAs(t, h, updated.Login)
		// Disabled users remain manageable, but disappear from the lesson selector.
		updated.Disabled = true
		updated = trainerResult(t, request(aa, "PUT", local(updated)+"/account", account(updated)), 200)
		status(t, request(coach, "GET", "/api/auth/me", nil), 401)
		if strings.Contains(request(aa, "GET", base+"?available=true", nil).Body.String(), u.ID) {
			t.Fatal("disabled selection")
		}
		updated.Disabled = false
		updated = trainerResult(t, request(aa, "PUT", local(updated)+"/account", account(updated)), 200)
		status(t, request(coach, "GET", "/api/auth/me", nil), 401)
		coach.cookies, coach.csrf = loginAs(t, h, updated.Login)
		nextPassword := "replacement-synthetic-password"
		updated = trainerResult(t, request(aa, "POST", local(updated)+"/password", map[string]any{"version": updated.Version, "password": nextPassword}), 200)
		status(t, request(coach, "GET", "/api/auth/me", nil), 401)
		status(t, call(h, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": updated.Login, "password": testPassword}), 401)
		status(t, call(h, "POST", "/api/auth/login", testOrigin, "", nil, map[string]string{"login": updated.Login, "password": nextPassword}), 200)
		var audit string
		if err := pool.QueryRow(ctx, `SELECT string_agg(coalesce(before_state::text,'')||coalesce(after_state::text,''),'') FROM pride.audit_events WHERE entity_id=$1`, u.ID).Scan(&audit); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{testPassword, nextPassword, updated.Login, u.DisplayName, "password_hash", hash} {
			if strings.Contains(audit, secret) {
				t.Fatal("secret or name in audit")
			}
		}
	})
	t.Run("shared account local revoke and owner restore", func(t *testing.T) {
		u := create("shared")
		coach := actor{}
		coach.cookies, coach.csrf = loginAs(t, h, u.Login)
		status(t, request(owner, "POST", "/api/staff/"+u.ID+"/trainer-assignments", map[string]any{"clubId": b, "version": u.Version}), 200)
		u.Version++
		list := request(aa, "GET", base, nil)
		status(t, list, 200)
		if strings.Contains(list.Body.String(), b) || strings.Contains(list.Body.String(), "trainerClubIds") {
			t.Fatal("foreign assignments disclosed")
		}
		var localList []Trainer
		_ = json.Unmarshal(list.Body.Bytes(), &localList)
		for _, item := range localList {
			if item.ID == u.ID && item.CanManageAccount {
				t.Fatal("shared account manageable")
			}
		}
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 403)
		status(t, request(aa, "POST", local(u)+"/password", map[string]any{"version": u.Version, "password": testPassword}), 403)
		// historical consumer references the author identity, never a removable role.
		execSQL(t, pool, `CREATE TABLE historical_author(staff_id uuid REFERENCES pride.staff_users(id) ON DELETE RESTRICT)`)
		execSQL(t, pool, `INSERT INTO historical_author VALUES($1)`, u.ID)
		status(t, request(aa, "POST", local(u)+"/revoke", map[string]any{"version": u.Version}), 200)
		u.Version++
		status(t, request(coach, "GET", "/api/auth/clubs/"+a, nil), 404)
		status(t, request(coach, "GET", "/api/auth/clubs/"+b, nil), 200)
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 404)
		var count int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM historical_author WHERE staff_id=$1`, u.ID).Scan(&count)
		if count != 1 {
			t.Fatal("historical author lost")
		}
		status(t, request(aa, "POST", base, map[string]any{"login": u.Login, "displayName": "Duplicate", "password": testPassword}), 409)
		u.DisplayName = "Owner renamed shared"
		u = trainerResult(t, request(owner, "PUT", "/api/staff/"+u.ID+"/account", account(u)), 200)
		status(t, request(owner, "POST", "/api/staff/"+u.ID+"/trainer-assignments", map[string]any{"clubId": a, "version": u.Version}), 200)
		status(t, request(coach, "GET", "/api/auth/clubs/"+a, nil), 200)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := RequireTrainer(ctx, tx, u.ID, a); err != nil {
			t.Fatal(err)
		}
		if err := RequireTrainer(ctx, tx, adminA, b); err != pgx.ErrNoRows {
			t.Fatal("wrong trainer accepted", err)
		}
	})
	t.Run("other roles cannot be escalated or modified by administrator", func(t *testing.T) {
		roster := request(owner, "GET", "/api/trainer-accounts", nil)
		status(t, roster, 200)
		status(t, request(owner, "POST", "/api/staff/"+adminB+"/trainer-assignments", map[string]any{"clubId": a, "version": 1}), 200)
		u := Trainer{ID: adminB, Login: "admin-b", DisplayName: "Admin B", Version: 2}
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 403)
		status(t, request(aa, "POST", local(u)+"/revoke", map[string]any{"version": 2}), 200)
		status(t, request(bb, "GET", "/api/auth/clubs/"+b, nil), 200)
		ownerID := queryID(t, pool, `SELECT id::text FROM pride.staff_users WHERE is_owner`)
		status(t, request(owner, "POST", "/api/staff/"+ownerID+"/trainer-assignments", map[string]any{"clubId": a, "version": 1}), 404)
		missing := request(aa, "PUT", base+"/not-an-id/account", account(u))
		foreign := request(bb, "PUT", local(u)+"/account", account(u))
		status(t, missing, 404)
		status(t, foreign, 404)
		if missing.Body.String() != foreign.Body.String() {
			t.Fatal("foreign existence leaked")
		}
	})
	t.Run("atomic audit failure and concurrent versions", func(t *testing.T) {
		u := create("race")
		coach := actor{}
		coach.cookies, coach.csrf = loginAs(t, h, u.Login)
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- request(aa, "PUT", local(u)+"/account", account(u)).Code }()
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for c := range codes {
			counts[c]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatal(counts)
		}
		u.Version++
		execSQL(t, pool, `CREATE FUNCTION pride.fail_trainer_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic audit failure'; END; $$; CREATE TRIGGER fail_trainer_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.fail_trainer_audit()`)
		status(t, request(aa, "POST", local(u)+"/revoke", map[string]any{"version": u.Version}), 503)
		status(t, request(aa, "POST", local(u)+"/password", map[string]any{"version": u.Version, "password": "never-persisted-password"}), 503)
		status(t, request(coach, "GET", "/api/auth/me", nil), 200)
		status(t, request(owner, "POST", "/api/staff/"+u.ID+"/trainer-assignments", map[string]any{"version": u.Version, "clubId": b}), 503)
		u.Login = "never-persisted-login"
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 503)
		status(t, request(aa, "POST", base, map[string]any{"login": "never-created", "displayName": "Never", "password": testPassword}), 503)
		var count int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pride.staff_users WHERE login IN ('never-created','never-persisted-login')`).Scan(&count)
		if count != 0 {
			t.Fatal("partial audit rollback")
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='trainer'`, u.ID).Scan(&count)
		if count != 1 {
			t.Fatal("partial revoke")
		}
		if !VerifyPassword(queryID(t, pool, `SELECT password_hash FROM pride.staff_users WHERE id=$1`, u.ID), testPassword) {
			t.Fatal("partial password")
		}
		execSQL(t, pool, `DROP TRIGGER fail_trainer_audit ON pride.audit_events; DROP FUNCTION pride.fail_trainer_audit()`)
	})
	t.Run("new shared assignment serializes with account permission", func(t *testing.T) {
		u := create("becomes-shared")
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, u.ID, b); err != nil {
			t.Fatal(err)
		}
		done := make(chan int, 1)
		go func() { done <- request(aa, "PUT", local(u)+"/account", account(u)).Code }()
		select {
		case c := <-done:
			t.Fatal("assignment lock bypassed", c)
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case c := <-done:
			if c != 403 {
				t.Fatal(c)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("blocked mutation")
		}
	})
	t.Run("live actor role revalidation after middleware", func(t *testing.T) {
		u := create("revocation-race")
		entered := make(chan struct{})
		resume := make(chan struct{})
		paused := s.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.SetPathValue("clubID", a)
			r.SetPathValue("staffID", u.ID)
			close(entered)
			<-resume
			s.changeTrainer(w, r)
		}))
		done := make(chan int, 1)
		go func() {
			done <- call(paused, "PUT", local(u)+"/account", testOrigin, aa.csrf, aa.cookies, account(u)).Code
		}()
		<-entered
		execSQL(t, pool, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, adminA)
		close(resume)
		if c := <-done; c != 404 {
			t.Fatal("revoked actor wrote", c)
		}
		execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, adminA, a)
	})
	t.Run("archive denies writes but permits local revoke", func(t *testing.T) {
		u := create("archive")
		execSQL(t, pool, `UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, a)
		status(t, request(aa, "POST", base, map[string]any{"login": "archived-new", "displayName": "New", "password": testPassword}), 409)
		status(t, request(aa, "PUT", local(u)+"/account", account(u)), 409)
		status(t, request(owner, "POST", "/api/staff/"+u.ID+"/trainer-assignments", map[string]any{"clubId": a, "version": u.Version}), 409)
		if body := request(aa, "GET", base+"?available=true", nil).Body.String(); body != "[]\n" {
			t.Fatal(body)
		}
		status(t, request(aa, "POST", local(u)+"/revoke", map[string]any{"version": u.Version}), 200)
	})
}
