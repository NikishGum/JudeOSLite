package main

import (
	"context"
	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	cfg, e := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	if cfg.ConnConfig.Database != "audit_browser" || cfg.ConnConfig.User != "test" || (cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "localhost") {
		log.Fatal("Only the disposable loopback audit_browser database with test user is allowed")
	}
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	must := func(e error) {
		if e != nil {
			log.Fatal(e)
		}
	}
	must(auth.CreateFirstOwner(ctx, p, "owner", "Synthetic Owner", "synthetic-browser-password"))
	var owner, a, b, archived string
	must(p.QueryRow(ctx, "SELECT id::text FROM pride.staff_users WHERE is_owner").Scan(&owner))
	for _, x := range []struct {
		name     string
		ptr      *string
		archived bool
	}{{"Audit A", &a, false}, {"Audit B", &b, false}, {"Audit Archive", &archived, true}} {
		must(p.QueryRow(ctx, "INSERT INTO pride.clubs(name,archived_at) VALUES($1,CASE WHEN $2 THEN now() ELSE NULL END) RETURNING id::text", x.name, x.archived).Scan(x.ptr))
	}
	for _, x := range []struct{ name, club, role string }{{"admin", a, "administrator"}, {"trainer", a, "trainer"}, {"archived-admin", archived, "administrator"}} {
		var u string
		must(p.QueryRow(ctx, "INSERT INTO pride.staff_users(login,display_name,password_hash) SELECT $1,$1,password_hash FROM pride.staff_users WHERE is_owner RETURNING id::text", x.name).Scan(&u))
		_, e = p.Exec(ctx, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,$3)", u, x.club, x.role)
		must(e)
	}
	for _, x := range []struct {
		club   any
		action string
		n      int
	}{{a, "created", 55}, {a, "updated", 3}, {b, "b_only", 2}, {archived, "archived", 1}, {nil, "network_only", 1}} {
		for range x.n {
			_, e = p.Exec(ctx, "INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,created_at,before_state,after_state,change_reason) VALUES($1,$2,'training_group',gen_random_uuid(),$3,'2026-10-12T12:00:00Z','{\"contact\":\"secret-contact\"}','{\"password\":\"secret-password\"}','private-reason')", x.club, owner, x.action)
			must(e)
		}
	}
}
