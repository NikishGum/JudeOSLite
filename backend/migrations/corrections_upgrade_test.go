package migrations

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestCorrectionsUpgradePreservesClosedMarks(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	name := fmt.Sprintf("corrections_upgrade_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e = admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); e != nil {
			t.Error(e)
		}
	}()
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.Database = name
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	// Establish pre-0010 migration checksums and genuine frozen, closed marks.
	if _, e = pool.Exec(ctx, `CREATE TABLE public.schema_migrations(version integer PRIMARY KEY CHECK (version > 0),checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); e != nil {
		t.Fatal(e)
	}
	ms, e := read()
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range ms[:9] {
		if _, e = pool.Exec(ctx, m.sql); e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `INSERT INTO public.schema_migrations(version,checksum) VALUES($1,$2)`, m.version, m.checksum); e != nil {
			t.Fatal(e)
		}
	}
	var club, user, group, card, source, session, record string
	id := func(q string, args ...any) string {
		t.Helper()
		var v string
		if e = pool.QueryRow(ctx, q, args...).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e = pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	club = id(`INSERT INTO pride.clubs(name) VALUES('Synthetic') RETURNING id::text`)
	user = id(`INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('synthetic','Synthetic','synthetic-hash') RETURNING id::text`)
	group = id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Synthetic') RETURNING id::text`, club)
	participant := id(`INSERT INTO pride.participants(full_name,birth_date) VALUES('Synthetic','1990-01-01') RETURNING id::text`)
	card = id(`INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2000-01-01','phone','synthetic') RETURNING id::text`, participant, club)
	source = id(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2000-01-01') RETURNING id::text`, club, group, card)
	session = id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,'2000-01-02','Europe/Minsk','2000-01-02 18:00+03','2000-01-02 19:00+03') RETURNING id::text`, club, group, user)
	record = id(`INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, club, session, group, card, source)
	exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=now() WHERE id=$1`, session)
	exec(`UPDATE pride.attendance_records SET state='absent',absence_reason='unknown',marked_by=$2,marked_at=now(),origin='close_default',version=2 WHERE id=$1`, record, user)
	exec(`UPDATE pride.training_sessions SET status='closed',closed_at=now() WHERE id=$1`, session)
	if e = Apply(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if e = Apply(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if e = Check(ctx, pool); e != nil {
		t.Fatal(e)
	}
	var preserved bool
	if e = pool.QueryRow(ctx, `SELECT a.state='absent' AND a.absence_reason='unknown' AND a.origin='close_default' AND a.marked_by=$2 AND a.version=2 AND a.change_reason IS NULL AND a.source_group_membership_id=$3 AND s.status='closed' AND s.roster_frozen_at IS NOT NULL FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE a.id=$1`, record, user, source).Scan(&preserved); e != nil || !preserved {
		t.Fatalf("closed snapshot lost: %v", e)
	}
	exec(`UPDATE pride.attendance_records SET state='present',absence_reason=NULL,origin='correction',change_reason='synthetic upgrade correction',version=3 WHERE id=$1`, record)
}
