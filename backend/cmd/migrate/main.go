package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func run() error {
	cfg, err := platform.LoadConfig(false)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return migrations.Apply(ctx, pool)
}

func main() {
	if err := run(); err != nil {
		// pgx errors can include connection strings; do not log them.
		log.Print("migration failed: verify DATABASE_URL, database availability and migration history")
		os.Exit(1)
	}
	log.Print("migrations applied")
}
