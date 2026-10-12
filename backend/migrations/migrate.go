package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

type migration struct {
	version             int
	name, sql, checksum string
}

func read() ([]migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var result []migration
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(prefix)
		if !ok || err != nil || v < 1 {
			return nil, fmt.Errorf("invalid migration name: %s", e.Name())
		}
		body, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(body)
		result = append(result, migration{v, e.Name(), string(body), hex.EncodeToString(hash[:])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for i, m := range result {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration versions must be contiguous from 1: %s", m.name)
		}
	}
	return result, nil
}

func applied(ctx context.Context, conn *pgx.Conn) (map[int]string, error) {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM public.schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]string{}
	for rows.Next() {
		var v int
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			return nil, err
		}
		result[v] = sum
	}
	return result, rows.Err()
}

func validate(ms []migration, done map[int]string, complete bool) error {
	for version, sum := range done {
		if version < 1 || version > len(ms) || ms[version-1].checksum != sum {
			return fmt.Errorf("migration %d is unknown or changed", version)
		}
		for prior := 1; prior < version; prior++ {
			if _, ok := done[prior]; !ok {
				return fmt.Errorf("migration %d is missing", prior)
			}
		}
	}
	if complete && len(done) != len(ms) {
		return fmt.Errorf("pending migrations; run the migrate command before server startup")
	}
	return nil
}

// Apply is only called by cmd/migrate. A session lock serializes runners;
// each SQL file and its checksum are committed in a single transaction.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := read()
	if err != nil {
		return err
	}
	c, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	if _, err = c.Exec(ctx, "SELECT pg_advisory_lock(149001)"); err != nil {
		return err
	}
	defer func() {
		// Close the session if unlock fails, so a pooled session never retains the lock.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := c.Exec(unlockCtx, "SELECT pg_advisory_unlock(149001)"); err != nil {
			_ = c.Conn().Close(unlockCtx)
		}
	}()
	if _, err = c.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.schema_migrations (
		version integer PRIMARY KEY CHECK (version > 0),
		checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	done, err := applied(ctx, c.Conn())
	if err != nil {
		return err
	}
	if err = validate(ms, done, false); err != nil {
		return err
	}
	for _, m := range ms {
		if _, ok := done[m.version]; ok {
			continue
		}
		tx, err := c.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, m.sql); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO public.schema_migrations(version, checksum) VALUES ($1, $2)", m.version, m.checksum)
		}
		if err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("migration %d failed: %w", m.version, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Check prevents starting a new binary against an absent, outdated or changed schema.
func Check(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := read()
	if err != nil {
		return err
	}
	c, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	done, err := applied(ctx, c.Conn())
	if err != nil {
		return err
	}
	return validate(ms, done, true)
}
