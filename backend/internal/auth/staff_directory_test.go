package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestStaffDirectoryIntegration(t *testing.T) {
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
	a := queryID(t, pool, `INSERT INTO pride.clubs(name) VALUES('Allowed A') RETURNING id::text`)
	b := queryID(t, pool, `INSERT INTO pride.clubs(name) VALUES('Private B') RETURNING id::text`)
	hash, _ := HashPassword(testPassword)
	newUser := func(login string) string {
		return queryID(t, pool, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,$1,$2) RETURNING id::text`, login, hash)
	}
	admin := newUser("admin")
	parent := newUser("parent-only")
	execSQL(t, pool, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, admin, a)
	family := queryID(t, pool, `INSERT INTO pride.families(club_id,name) VALUES($1,'Secret family name') RETURNING id::text`, a)
	execSQL(t, pool, `INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, family, parent)
	type actor struct {
		cookies []*http.Cookie
		csrf    string
	}
	login := func(name string) actor { c, csrf := loginAs(t, h, name); return actor{c, csrf} }
	owner, local, parentActor := login("owner"), login("admin"), login("parent-only")
	request := func(c actor, method, path string, body any) *httptest.ResponseRecorder {
		return call(h, method, path, testOrigin, c.csrf, c.cookies, body)
	}
	get := func(c actor) StaffDirectory {
		w := request(c, "GET", "/api/staff-directory", nil)
		status(t, w, 200)
		if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), family) || strings.Contains(w.Body.String(), "Secret family") {
			t.Fatal("directory privacy", w.Body.String())
		}
		var d StaffDirectory
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	find := func(d StaffDirectory, id string) StaffCard {
		for _, person := range d.Staff {
			if person.ID == id {
				return person
			}
		}
		t.Fatalf("staff %s absent", id)
		return StaffCard{}
	}
	base := "/api/clubs/" + a + "/trainers"
	created := trainerResult(t, request(local, "POST", base, map[string]any{"login": "coach", "displayName": "Synthetic Coach", "password": testPassword}), 201)
	coach := login("coach")
	assignBase := "/api/staff/" + created.ID + "/administrator-assignments"
	assignment := func(version int, club string) any { return map[string]any{"clubId": club, "version": version} }
	t.Run("one snapshot owner admin scope and parent exclusion", func(t *testing.T) {
		status(t, call(h, "GET", "/api/staff-directory", "", "", nil, nil), 401)
		status(t, request(coach, "GET", "/api/staff-directory", nil), 403)
		status(t, request(parentActor, "GET", "/api/staff-directory", nil), 403)
		status(t, request(parentActor, "GET", "/api/staff-candidates?query=parent", nil), 403)
		status(t, request(local, "GET", "/api/staff-candidates?query=parent", nil), 403)
		d := get(owner)
		if len(d.Staff) != 2 || !d.CanAssignExisting || len(d.Clubs) != 2 {
			t.Fatal("owner roster", d)
		}
		for _, person := range d.Staff {
			if person.ID == parent {
				t.Fatal("parent-only is not an employee")
			}
		}
		d = get(local)
		if len(d.Staff) != 1 || d.CanAssignExisting || len(d.Clubs) != 1 || d.Clubs[0].ID != a || !d.Staff[0].CanManageAccount {
			t.Fatal("local scope", d)
		}
		status(t, request(local, "POST", assignBase, assignment(1, a)), 403)
		status(t, request(owner, "POST", assignBase+"/revoke", assignment(1, "invalid-id")), 400)
		status(t, call(h, "POST", assignBase, testOrigin, "", owner.cookies, assignment(1, a)), 403)
	})
	t.Run("point assignment preserves every other role and versions", func(t *testing.T) {
		status(t, request(owner, "POST", assignBase, assignment(1, b)), 200)
		card := find(get(owner), created.ID)
		if card.Version != 2 || len(card.Assignments) != 2 {
			t.Fatal("assignment snapshot", card)
		}
		// A duplicate with a current version is a no-op; stale duplicate conflicts.
		status(t, request(owner, "POST", assignBase, assignment(2, b)), 200)
		status(t, request(owner, "POST", assignBase, assignment(1, b)), 409)
		if find(get(owner), created.ID).Version != 2 {
			t.Fatal("duplicate changed version")
		}
		localCard := find(get(local), created.ID)
		if localCard.CanManageAccount || len(localCard.Assignments) != 1 || strings.Contains(request(local, "GET", "/api/staff-directory", nil).Body.String(), b) {
			t.Fatal("shared account/foreign assignment leaked", localCard)
		}
		status(t, request(local, "PUT", base+"/"+created.ID+"/account", map[string]any{"login": "coach", "displayName": "Changed", "disabled": true, "version": 2}), 403)
		status(t, request(owner, "POST", assignBase+"/revoke", assignment(2, b)), 200)
		card = find(get(owner), created.ID)
		if card.Version != 3 || len(card.Assignments) != 1 || card.Assignments[0].Role != "trainer" || card.Assignments[0].ClubID != a {
			t.Fatal("trainer lost", card)
		}
		status(t, request(coach, "GET", "/api/auth/clubs/"+b, nil), 404)
		status(t, request(coach, "GET", "/api/auth/clubs/"+a, nil), 200)
	})
	t.Run("mixed parent is staff but account remains owner only", func(t *testing.T) {
		execSQL(t, pool, `INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, family, created.ID)
		card := find(get(local), created.ID)
		if card.CanManageAccount {
			t.Fatal("mixed account editable by admin")
		}
		status(t, request(local, "POST", base+"/"+created.ID+"/password", map[string]any{"password": testPassword, "version": card.Version}), 403)
		if !find(get(owner), created.ID).CanManageAccount {
			t.Fatal("owner cannot manage mixed account")
		}
	})
	t.Run("former employees survive last role and can return", func(t *testing.T) {
		status(t, request(local, "POST", base+"/"+created.ID+"/revoke", map[string]any{"version": 3}), 200)
		card := find(get(owner), created.ID)
		if card.Version != 4 || len(card.Assignments) != 0 || card.Disabled {
			t.Fatal("former account disappeared", card)
		}
		if len(get(local).Staff) != 0 {
			t.Fatal("local admin retains revoked employee")
		}
		status(t, request(coach, "GET", "/api/auth/me", nil), 200)
		status(t, request(coach, "GET", "/api/auth/clubs/"+a, nil), 200) // explicit family access survives
		status(t, request(owner, "POST", assignBase, assignment(4, a)), 200)
		card = find(get(owner), created.ID)
		if card.Version != 5 || len(card.Assignments) != 1 || card.Assignments[0].Role != "administrator" {
			t.Fatal("former staff restoration", card)
		}
	})
	t.Run("archive owner protection atomic audit rollback and races", func(t *testing.T) {
		ownerID := queryID(t, pool, `SELECT id::text FROM pride.staff_users WHERE is_owner`)
		status(t, request(owner, "POST", "/api/staff/"+ownerID+"/administrator-assignments", assignment(1, b)), 404)
		execSQL(t, pool, `UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, b)
		status(t, request(owner, "POST", assignBase, assignment(5, b)), 409)
		execSQL(t, pool, `UPDATE pride.clubs SET archived_at=NULL WHERE id=$1`, b)
		execSQL(t, pool, `CREATE FUNCTION pride.fail_staff_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic audit failure'; END; $$; CREATE TRIGGER fail_staff_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.fail_staff_audit()`)
		status(t, request(owner, "POST", assignBase, assignment(5, b)), 503)
		status(t, request(owner, "POST", assignBase+"/revoke", assignment(5, a)), 503)
		if card := find(get(owner), created.ID); card.Version != 5 || len(card.Assignments) != 1 {
			t.Fatal("partial audit failure", card)
		}
		execSQL(t, pool, `DROP TRIGGER fail_staff_audit ON pride.audit_events; DROP FUNCTION pride.fail_staff_audit()`)
		codes := make(chan int, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- request(owner, "POST", assignBase, assignment(5, b)).Code }()
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatal("concurrent versions", counts)
		}
		var leaked bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pride.audit_events WHERE entity_id=$1 AND (after_state::text LIKE '%Synthetic%' OR after_state::text LIKE '%password%' OR after_state::text LIKE '%coach%'))`, created.ID).Scan(&leaked); err != nil || leaked {
			t.Fatal("audit privacy", err)
		}
	})
	t.Run("directory never combines versions and roles from different snapshots", func(t *testing.T) {
		u := trainerResult(t, request(local, "POST", base, map[string]any{"login": "snapshot", "displayName": "Synthetic Snapshot", "password": testPassword}), 201)
		done := make(chan bool, 1)
		codes := make(chan int, 20)
		go func() {
			for version := 1; version <= 20; version++ {
				path := "/api/staff/" + u.ID + "/administrator-assignments"
				if version%2 == 0 {
					path += "/revoke"
				}
				codes <- request(owner, "POST", path, assignment(version, b)).Code
			}
			done <- true
		}()
		for {
			card := find(get(owner), u.ID)
			want := 1
			if card.Version%2 == 0 {
				want = 2
			}
			if len(card.Assignments) != want {
				t.Fatal("inconsistent version and role snapshot", card)
			}
			select {
			case <-done:
				for i := 0; i < 20; i++ {
					if code := <-codes; code != 200 {
						t.Fatal("snapshot writer", code)
					}
				}
				return
			default:
			}
		}
	})
	t.Run("live read role check after middleware", func(t *testing.T) {
		entered, resume, done := make(chan struct{}), make(chan struct{}), make(chan int, 1)
		paused := s.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-resume
			s.staffDirectory(w, r)
		}))
		go func() { done <- call(paused, "GET", "/api/staff-directory", "", "", local.cookies, nil).Code }()
		<-entered
		execSQL(t, pool, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, admin)
		close(resume)
		if code := <-done; code != 403 {
			t.Fatal("revoked administrator read directory", code)
		}
	})
	t.Run("live read and write actor/session checks after middleware", func(t *testing.T) {
		for _, write := range []bool{false, true} {
			entered, resume, done := make(chan struct{}), make(chan struct{}), make(chan int, 1)
			paused := s.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-resume
				if write {
					r.SetPathValue("staffID", created.ID)
					s.administratorAssignment(w, r)
				} else {
					s.staffDirectory(w, r)
				}
			}))
			actor := login("owner")
			go func() {
				done <- call(paused, "POST", assignBase, testOrigin, actor.csrf, actor.cookies, assignment(6, a)).Code
			}()
			<-entered
			execSQL(t, pool, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id=(SELECT id FROM pride.staff_users WHERE is_owner)`)
			close(resume)
			if code := <-done; code != 404 {
				t.Fatal("revoked session accepted", code)
			}
		}
	})
	t.Run("owner explicitly converts existing parent without duplicate or family disclosure", func(t *testing.T) {
		owner = login("owner")
		status(t, request(owner, "GET", "/api/staff-candidates?query=pa", nil), 400)
		w := request(owner, "GET", "/api/staff-candidates?query=parent-only", nil)
		status(t, w, 200)
		var found []StaffCard
		if err := json.Unmarshal(w.Body.Bytes(), &found); err != nil || len(found) != 1 || found[0].ID != parent || len(found[0].Assignments) != 0 || strings.Contains(w.Body.String(), family) || strings.Contains(w.Body.String(), "Secret family") {
			t.Fatal("candidate lookup", w.Body.String(), err)
		}
		for _, person := range get(owner).Staff {
			if person.ID == parent {
				t.Fatal("lookup itself added a staff identity")
			}
		}
		status(t, request(owner, "POST", "/api/staff/"+parent+"/trainer-assignments", assignment(found[0].Version, a)), 200)
		if card := find(get(owner), parent); len(card.Assignments) != 1 || card.Login != "parent-only" {
			t.Fatal("existing parent conversion", card)
		}
		var links int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pride.family_access WHERE user_id=$1`, parent).Scan(&links); err != nil || links != 1 {
			t.Fatal("parent access lost", err)
		}
		for i := 0; i < 35; i++ {
			newUser(fmt.Sprintf("candidate-bound-%02d", i))
		}
		w = request(owner, "GET", "/api/staff-candidates?query=candidate-bound", nil)
		status(t, w, 200)
		if err := json.Unmarshal(w.Body.Bytes(), &found); err != nil || len(found) != 30 {
			t.Fatal("unbounded candidates", err, len(found))
		}
		w = request(owner, "GET", "/api/staff-candidates?query=%25%25%25", nil)
		status(t, w, 200)
		if strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal("wildcards were not literal", w.Body.String())
		}
	})
}
