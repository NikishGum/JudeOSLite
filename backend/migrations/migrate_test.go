package migrations

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required for real PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	name := fmt.Sprintf("judeos_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if Check(ctx, pool) == nil {
		t.Fatal("uninitialized database accepted")
	}
	// Force the SQL file to fail; its history row must not be committed.
	if _, err = pool.Exec(ctx, "CREATE SCHEMA pride"); err != nil {
		t.Fatal(err)
	}
	if Apply(ctx, pool) == nil {
		t.Fatal("conflicting schema accepted")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial migration history: %d %v", count, err)
	}
	if _, err = pool.Exec(ctx, "DROP SCHEMA pride"); err != nil {
		t.Fatal(err)
	}
	// Upgrade a live V1-02 schema, including identities, archive, roles and sessions.
	ms, err := read()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	for _, m := range ms[:2] {
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO public.schema_migrations(version,checksum) VALUES($1,$2)", m.version, m.checksum); err != nil {
			t.Fatal(err)
		}
	}
	var clubID, userID string
	if err = tx.QueryRow(ctx, "INSERT INTO pride.clubs(name,address,archived_at) VALUES('Existing hall','Existing address',now()) RETURNING id::text").Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES('existing','Existing staff','synthetic-hash') RETURNING id::text").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')", userID, clubID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pride.staff_sessions(staff_user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", userID, make([]byte, 32), make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Go(func() { errs <- Apply(ctx, pool) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Check(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var preserved bool
	if err = pool.QueryRow(ctx, `SELECT c.name='Existing hall' AND c.address='Existing address' AND c.archived_at IS NOT NULL AND c.version=1 AND u.password_hash='synthetic-hash' AND u.version=1
 FROM pride.clubs c JOIN pride.staff_club_roles r ON r.club_id=c.id JOIN pride.staff_users u ON u.id=r.staff_user_id JOIN pride.staff_sessions s ON s.staff_user_id=u.id
 WHERE c.id=$1 AND u.id=$2 AND r.role='administrator' AND s.revoked_at IS NULL`, clubID, userID).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("V1-02 data lost during upgrade: %v", err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&count); err != nil || count != len(ms) {
		t.Fatalf("duplicate migration history: %d %v", count, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE public.schema_migrations SET checksum='changed'"); err != nil {
		t.Fatal(err)
	}
	if Apply(ctx, pool) == nil || Check(ctx, pool) == nil {
		t.Fatal("changed migration accepted")
	}
}
