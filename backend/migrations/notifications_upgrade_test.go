package migrations

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNotificationsUpgradeFrom0013(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	db := fmt.Sprintf("inbox_upgrade_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{db}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH(FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = db
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(q string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, q, args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	ms, err := read()
	if err != nil {
		t.Fatal(err)
	}
	exec(`CREATE TABLE public.schema_migrations(version integer PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`)
	for _, m := range ms[:13] {
		exec(m.sql)
		exec(`INSERT INTO public.schema_migrations(version,checksum) VALUES($1,$2)`, m.version, m.checksum)
	}
	u := id(`INSERT INTO pride.staff_users(login,display_name,password_hash,is_owner) VALUES('existing','Existing','unchanged',true) RETURNING id::text`)
	c := id(`INSERT INTO pride.clubs(name) VALUES('Existing') RETURNING id::text`)
	f := id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Existing family') RETURNING id::text`, c)
	exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, f, u)
	exec(`INSERT INTO pride.staff_sessions(staff_user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')`, u, make([]byte, 32), make([]byte, 32))
	p := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES('Existing child','2015-01-01') RETURNING id::text`)
	m := id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2020-01-01','phone','unchanged') RETURNING id::text`, c, p)
	d := id(`INSERT INTO pride.member_documents(club_id,membership_id,name,kind,filename,media_type,content,created_by) VALUES($1,$2,'Private','other','file.pdf','application/pdf',$3,$4) RETURNING id::text`, c, m, []byte("%PDF-1.7\n%%EOF"), u)
	exec(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,'family_updated','{"version":1}')`, c, u, f)
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if id(`SELECT count(*)::text FROM pride.notifications`) != "0" {
		t.Fatal("upgrade backfilled old events")
	}
	exec(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'family',$3,'family_updated','{"version":2}')`, c, u, f)
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Check(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if id(`SELECT count(*)::text FROM pride.notifications`) != "1" {
		t.Fatal("rerun duplicated or lost event")
	}
	var preserved bool
	err = pool.QueryRow(ctx, `SELECT u.password_hash='unchanged' AND d.content=$2 AND s.revoked_at IS NULL AND a.family_id=$3::uuid FROM pride.member_documents d JOIN pride.staff_users u ON u.id=d.created_by JOIN pride.staff_sessions s ON s.staff_user_id=u.id JOIN pride.family_access a ON a.user_id=u.id WHERE d.id=$1`, d, []byte("%PDF-1.7\n%%EOF"), f).Scan(&preserved)
	if err != nil || !preserved {
		t.Fatalf("upgrade changed private records/session: %v", err)
	}
}
