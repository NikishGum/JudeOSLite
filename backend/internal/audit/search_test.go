package audit

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
)

func TestSearchQueryValidation(t *testing.T) {
	q, err := parseQuery(url.Values{"q": {"  ЁЖ #ABC исправлена "}, "actorId": {"ABCDEFAB-1234-1234-1234-ABCDEFABCDEF"}})
	if err != nil || q.search != "abc еж исправлена" || q.actor != "abcdefab-1234-1234-1234-abcdefabcdef" {
		t.Fatal(q, err)
	}
	same, _ := parseQuery(url.Values{"q": {"ИСПРАВЛЕНА abc еж"}, "actorId": {q.actor}})
	if same.fingerprint != q.fingerprint {
		t.Fatal("equivalent search has different fingerprint")
	}
	for _, values := range []url.Values{{"q": {"\x00"}}, {"q": {string([]byte{0xff})}}, {"q": {strings.Repeat("я", 161)}}, {"q": {"one", "two"}}, {"actorId": {"name"}}, {"actorId": {"a", "b"}}} {
		if _, err := parseQuery(values); err == nil {
			t.Fatalf("accepted %v", values)
		}
	}
	if _, err := parseQuery(url.Values{"q": {strings.Repeat("я", 160)}}); err != nil {
		t.Fatal(err)
	}
}

func TestAuditSearchAuthorsAndTotal(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if err := auth.CreateFirstOwner(ctx, p, "owner", "Владелец", testPassword); err != nil {
		t.Fatal(err)
	}
	ownerID := id(t, p, "SELECT id::text FROM pride.staff_users WHERE is_owner")
	a := id(t, p, "INSERT INTO pride.clubs(name) VALUES('Брест · Ёлка') RETURNING id::text")
	b := id(t, p, "INSERT INTO pride.clubs(name) VALUES('Чужой филиал') RETURNING id::text")
	users := map[string]string{}
	for _, name := range []string{"admin", "multi", "trainer", "parent", "former", "namesake", "foreign", "no-audit"} {
		users[name] = id(t, p, `INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$2,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text`, name, map[string]string{"former": "Ольга Калиновская", "namesake": "Ольга Калиновская", "foreign": "Чужой автор"}[name]+name)
	}
	exec(t, p, "UPDATE pride.staff_users SET display_name='Ольга Калиновская' WHERE id=ANY($1::uuid[])", []string{users["former"], users["namesake"]})
	exec(t, p, "UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1", users["former"])
	for _, role := range []struct{ user, club, role string }{{"admin", a, "administrator"}, {"multi", a, "administrator"}, {"multi", b, "administrator"}, {"trainer", a, "trainer"}} {
		exec(t, p, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)", users[role.user], role.club, role.role)
	}
	// A parent without staff assignments must not acquire audit access.
	family := id(t, p, "INSERT INTO pride.families(club_id,name) VALUES($1,'Synthetic audit family') RETURNING id::text", a)
	exec(t, p, "INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)", family, users["parent"])
	add := func(club any, actor, action string) string {
		return id(t, p, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,created_at,before_state,change_reason) VALUES($1,$2,'attendance_record',gen_random_uuid(),$3,'2026-10-12T12:00Z','{"secret":"hidden-payload"}','hidden-reason') RETURNING id::text`, club, actor, action)
	}
	for range 55 {
		add(a, ownerID, "created")
	}
	target := add(a, users["former"], "corrected")
	entity := id(t, p, "SELECT entity_id::text FROM pride.audit_events WHERE id=$1", target)
	add(a, users["namesake"], "future_action")
	add(b, users["foreign"], "foreign_action")
	add(nil, ownerID, "network_only")
	access, err := auth.New(p, auth.Config{Origin: testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	service := New(p, access)
	h := access.Handler(service.Handler(http.NotFoundHandler()))
	owner, admin, multi := login(t, h, "owner"), login(t, h, "admin"), login(t, h, "multi")
	read := func(c client, q string) Page { return parse[Page](t, c.get("/api/audit/events?"+q)) }
	search := func(q string) string { return "q=" + url.QueryEscape(q) }
	t.Run("all words aliases ids literal characters and unknown codes", func(t *testing.T) {
		for _, q := range []string{"Калиновская исправлена", "ИСПРАВЛЕНА КАЛИНОВСКАЯ", "елка исправлена", "ЁЛКА corrected", "отметка corrected", target, "#" + target[:8], "#" + entity[:8], entity} {
			got := read(admin, search(q))
			if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != target {
				t.Fatalf("%q: %+v", q, got)
			}
			if got.Items[0].ActionLabel != actionCatalog["corrected"].Label {
				t.Fatal(got)
			}
		}
		if got := read(admin, search("_")); got.Total != 57 {
			t.Fatal("underscore must match the literal entity codes", got.Total)
		}
		for _, q := range []string{"%", "__", "\\", "' OR true --", "hidden-payload", "hidden-reason", "Чужой"} {
			if got := read(admin, search(q)); got.Total != 0 || len(got.Items) != 0 {
				t.Fatalf("literal/private %q matched %+v", q, got)
			}
		}
		if got := read(admin, search("future_action")); got.Total != 1 || got.Items[0].ActionLabel != "future_action" {
			t.Fatal(got)
		}
		exact := "clubId=" + a + "&action=corrected&actorId=" + users["former"] + "&from=2026-10-12&until=2026-10-12&" + search("Калиновская")
		if got := read(admin, exact); got.Total != 1 || len(got.Items) != 1 {
			t.Fatal(got)
		}
		if got := read(admin, "actorId="+users["foreign"]); got.Total != 0 {
			t.Fatal("foreign actor leaks total", got)
		}
	})
	t.Run("authors only from allowed audit including disabled and namesakes", func(t *testing.T) {
		f := parse[Filters](t, admin.get("/api/audit/filters"))
		seen := map[string]Actor{}
		for _, actor := range f.Actors {
			seen[actor.ID] = actor
		}
		if len(seen) != 3 || !seen[users["former"]].Disabled || seen[users["former"]].Name != seen[users["namesake"]].Name {
			t.Fatal(f)
		}
		for _, denied := range []string{"foreign", "no-audit", "parent", "trainer"} {
			if _, ok := seen[users[denied]]; ok {
				t.Fatal("author leak", denied)
			}
		}
		if f.ActionLabels["corrected"] != actionCatalog["corrected"].Label {
			t.Fatal(f)
		}
		if got := read(multi, search("Чужой")); got.Total != 1 {
			t.Fatal(got)
		}
		if got := read(owner, "clubId=network"); got.Total != 1 {
			t.Fatal(got)
		}
		exec(t, p, "UPDATE pride.clubs SET archived_at=now() WHERE id=$1", a)
		if got := read(admin, "actorId="+users["former"]); got.Total != 1 || !got.Items[0].ClubArchived {
			t.Fatal(got)
		}
	})
	t.Run("cursor includes search and actor count ignores cursor", func(t *testing.T) {
		first := read(admin, search("создание"))
		if first.Total != 55 || len(first.Items) != 50 || first.NextCursor == "" {
			t.Fatal(first)
		}
		for _, condition := range []string{search("создание") + "&actorId=" + users["former"], search("Калиновская")} {
			status(t, admin.get("/api/audit/events?"+condition+"&cursor="+url.QueryEscape(first.NextCursor)), 400)
		}
		second := read(admin, search("СОЗДАНИЕ")+"&cursor="+url.QueryEscape(first.NextCursor))
		if second.Total != 55 || len(second.Items) != 5 || second.NextCursor != "" {
			t.Fatal(second)
		}
		exec(t, p, "DELETE FROM pride.audit_events WHERE id IN (SELECT id FROM pride.audit_events WHERE action='created' ORDER BY created_at DESC,id DESC OFFSET 50)")
		empty := read(admin, search("создание")+"&cursor="+url.QueryEscape(first.NextCursor))
		if empty.Total != 50 || len(empty.Items) != 0 {
			t.Fatal(empty)
		}
	})
	t.Run("one snapshot during concurrent insert", func(t *testing.T) {
		var writer sync.WaitGroup
		writer.Add(1)
		go func() {
			defer writer.Done()
			for range 20 {
				_, _ = p.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action) VALUES($1,$2,'training_group',gen_random_uuid(),'snapshot')`, a, ownerID)
			}
		}()
		for range 20 {
			got := read(admin, "action=snapshot")
			if int64(len(got.Items)) != got.Total {
				t.Fatal("items/total snapshots differ", got)
			}
		}
		writer.Wait()
	})
	t.Run("trainer parent and live scope revoke cover metadata search and counts", func(t *testing.T) {
		for _, name := range []string{"trainer", "parent"} {
			c := login(t, h, name)
			status(t, c.get("/api/audit/filters"), 403)
			status(t, c.get("/api/audit/events?q=Калиновская"), 403)
		}
		exec(t, p, "DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2", users["multi"], b)
		if got := read(multi, search("Чужой")); got.Total != 0 {
			t.Fatal(got)
		}
		f := parse[Filters](t, multi.get("/api/audit/filters"))
		for _, actor := range f.Actors {
			if actor.ID == users["foreign"] {
				t.Fatal("revoked author retained", f)
			}
		}
		exec(t, p, "DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1", users["admin"])
		status(t, admin.get("/api/audit/filters"), 403)
		status(t, admin.get("/api/audit/events?q=Калиновская"), 403)
	})
}
