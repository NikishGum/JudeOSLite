package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/audit"
	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/participants"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run() error {
	cfg, err := platform.LoadConfig(true)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	if err := migrations.Check(ctx, pool); err != nil {
		cancel()
		return errors.New("database unavailable or migrations pending/changed; run migrate")
	}
	cancel()
	var stopping atomic.Bool
	handler, err := platform.NewHandler(cfg.StaticDir, func(ctx context.Context) error {
		if stopping.Load() {
			return errors.New("shutting down")
		}
		return pool.Ping(ctx)
	})
	if err != nil {
		return err
	}
	insecureLocal, err := strconv.ParseBool(envDefault("AUTH_INSECURE_LOCAL", "false"))
	if err != nil {
		return errors.New("AUTH_INSECURE_LOCAL must be true or false")
	}
	authService, err := auth.New(pool, auth.Config{Origin: envDefault("APP_ORIGIN", "https://localhost"), InsecureLocal: insecureLocal})
	if err != nil {
		return err
	}
	scheduleService := schedule.New(pool, authService)
	handler = audit.New(pool, authService).Handler(handler)
	handler = authService.Handler(sessions.New(pool, authService).Handler(scheduleService.Handler(groups.New(pool, authService).Handler(participants.New(pool, authService).Handler(clubs.New(pool, authService).Handler(handler))))))
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return errors.New("cannot listen on HTTP_ADDR")
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	jobs, stopJobs := context.WithCancel(signals)
	jobsDone := make(chan struct{})
	defer func() { stopJobs(); <-jobsDone }()
	go func() {
		defer close(jobsDone)
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			generation, cancel := context.WithTimeout(jobs, 45*time.Second)
			if err := scheduleService.Generate(generation, time.Now()); err != nil && jobs.Err() == nil {
				log.Print("schedule generation failed; will retry")
			}
			cancel()
			select {
			case <-jobs.Done():
				return
			case <-tick.C:
			}
		}
	}()
	result := make(chan error, 1)
	go func() { result <- srv.Serve(listener) }()
	log.Print("server listening")
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-signals.Done():
		stopping.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
			return errors.New("HTTP shutdown timed out")
		}
		log.Print("server stopped")
		return nil
	}
}

func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/readyz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ready")
	}
	return nil
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if healthcheck() != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
