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

func TestBillingUpgradeFrom0011(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	dbname := fmt.Sprintf("billing_upgrade_%d", time.Now().UnixNano())
	name := pgx.Identifier{dbname}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(ctx, "DROP DATABASE "+name+" WITH(FORCE)"); e != nil {
			t.Error(e)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = dbname
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	id := func(q string, args ...any) string {
		t.Helper()
		var s string
		if e := pool.QueryRow(ctx, q, args...).Scan(&s); e != nil {
			t.Fatal(e)
		}
		return s
	}
	ms, err := read()
	if err != nil {
		t.Fatal(err)
	}
	exec(`CREATE TABLE public.schema_migrations(version integer PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`)
	for _, m := range ms[:11] {
		exec(m.sql)
		exec(`INSERT INTO public.schema_migrations(version,checksum) VALUES($1,$2)`, m.version, m.checksum)
	}
	club := id(`INSERT INTO pride.clubs(name) VALUES('Existing') RETURNING id::text`)
	user := id(`INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('existing','Existing','unchanged-hash') RETURNING id::text`)
	exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, user, club)
	exec(`INSERT INTO pride.staff_sessions(staff_user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')`, user, make([]byte, 32), make([]byte, 32))
	p := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES('Existing Child','2015-01-01') RETURNING id::text`)
	member := id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value,archived_at) VALUES($1,$2,'2020-01-01','phone','unchanged-contact',now()) RETURNING id::text`, club, p)
	family := id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Explicit Family') RETURNING id::text`, club)
	exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, family, club, member)
	exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, family, user)
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	payment := id(`INSERT INTO pride.payments(club_id,membership_id,request_id,request_body,description,amount_minor,status,created_by) VALUES($1,$2,gen_random_uuid(),'{}','Invoice',12345,'pending',$3) RETURNING id::text`, club, member, user)
	exec(`INSERT INTO pride.subscriptions(club_id,membership_id,payment_id,plan_name,visits,days,valid_from,valid_until) VALUES($1,$2,$3,'Snapshot',8,30,'2026-10-01','2026-10-31')`, club, member, payment)
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Check(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var ok bool
	err = pool.QueryRow(ctx, `SELECT m.contact_value='unchanged-contact' AND m.archived_at IS NOT NULL AND m.version=1 AND u.password_hash='unchanged-hash' AND r.role='trainer' AND s.revoked_at IS NULL AND f.version=1 FROM pride.club_memberships m JOIN pride.family_children fc ON fc.membership_id=m.id JOIN pride.families f ON f.id=fc.family_id JOIN pride.family_access a ON a.family_id=f.id JOIN pride.staff_users u ON u.id=a.user_id JOIN pride.staff_sessions s ON s.staff_user_id=u.id JOIN pride.staff_club_roles r ON r.staff_user_id=u.id WHERE m.id=$1 AND u.id=$2`, member, user).Scan(&ok)
	if err != nil || !ok {
		t.Fatalf("upgrade/rerun lost existing data: %v", err)
	}
	err = pool.QueryRow(ctx, `SELECT p.status='pending' AND p.amount_minor=12345 AND p.version=1 AND s.plan_name='Snapshot' AND s.visits=8 AND s.days=30 AND s.valid_until='2026-10-31'::date FROM pride.payments p JOIN pride.subscriptions s ON s.payment_id=p.id WHERE p.id=$1`, payment).Scan(&ok)
	if err != nil || !ok {
		t.Fatalf("rerun lost finance/snapshot: %v", err)
	}
	other := id(`INSERT INTO pride.clubs(name) VALUES('Other') RETURNING id::text`)
	otherPerson := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES('Other Child','2015-01-01') RETURNING id::text`)
	otherMember := id(`INSERT INTO pride.club_memberships(club_id,participant_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2020-01-01','phone','other-contact') RETURNING id::text`, other, otherPerson)
	_, err = pool.Exec(ctx, `INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, family, other, otherMember)
	if err == nil {
		t.Fatal("cross-club family link accepted")
	}
	_, err = pool.Exec(ctx, `INSERT INTO pride.payments(club_id,membership_id,request_id,request_body,description,amount_minor,status,created_by) VALUES($1,$2,gen_random_uuid(),'{}','Foreign invoice',1,'pending',$3)`, club, otherMember, user)
	if err == nil {
		t.Fatal("cross-club payment accepted")
	}
	second := id(`INSERT INTO pride.payments(club_id,membership_id,request_id,request_body,description,amount_minor,status,created_by) VALUES($1,$2,gen_random_uuid(),'{}','Second invoice',1,'pending',$3) RETURNING id::text`, club, member, user)
	_, err = pool.Exec(ctx, `INSERT INTO pride.subscriptions(club_id,membership_id,payment_id,plan_name,visits,days,valid_from,valid_until) VALUES($1,$2,$3,'Foreign snapshot',1,1,'2026-11-01','2026-11-02')`, other, otherMember, second)
	if err == nil {
		t.Fatal("subscription attached to another child's payment")
	}
}
