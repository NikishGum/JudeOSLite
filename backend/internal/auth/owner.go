package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateFirstOwner is a local bootstrap, never a public registration endpoint.
func CreateFirstOwner(ctx context.Context, pool *pgxpool.Pool, login, name, password string) error {
	login, err := NormalizeLogin(login)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 200 {
		return errors.New("display name must contain 1–200 characters")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.New("cannot start owner creation")
	}
	defer tx.Rollback(context.Background())
	// Serializes two local bootstrap commands, independently of the migration runner.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(150001)"); err != nil {
		return errors.New("cannot lock owner creation")
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pride.staff_users WHERE is_owner)").Scan(&exists); err != nil {
		return errors.New("cannot check owner; apply migrations first")
	}
	if exists {
		return errors.New("owner already exists; bootstrap refused")
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pride.staff_users(login, display_name, password_hash, is_owner) VALUES ($1,$2,$3,true)", login, name, hash); err != nil {
		return errors.New("cannot create owner; check login availability")
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("cannot commit owner creation")
	}
	return nil
}
