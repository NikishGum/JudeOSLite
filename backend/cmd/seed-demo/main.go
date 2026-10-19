// seed-demo is a local administrative command; it is never an HTTP endpoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/demodata"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

func run() error {
	loc, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		return err
	}
	date := flag.String("date", time.Now().In(loc).Format(time.DateOnly), "anchor date in Europe/Minsk, YYYY-MM-DD")
	apply := flag.Bool("apply", false, "explicitly add the fictional dataset to DATABASE_URL")
	accounts := flag.Bool("list-accounts", false, "print all demo logins without passwords or a database connection")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected arguments; use --help")
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if *accounts {
		if *apply {
			return errors.New("--list-accounts cannot be combined with --apply")
		}
		return enc.Encode(demodata.Accounts())
	}
	preview, err := demodata.Preview(*date)
	if err != nil {
		return err
	}
	if !*apply {
		fmt.Fprintln(os.Stderr, "Preview only. Use --apply to add this fictional network; existing data is preserved.")
		return enc.Encode(preview)
	}
	cfg, err := platform.LoadConfig(false)
	if err != nil {
		return err
	}
	password := []byte(os.Getenv("SEED_DEMO_PASSWORD"))
	os.Unsetenv("SEED_DEMO_PASSWORD")
	defer func() { clear(password) }()
	if len(password) == 0 {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("hidden terminal input or SEED_DEMO_PASSWORD is required; no password command-line option")
		}
		fmt.Fprint(os.Stderr, "Пароль всех демо-аккаунтов (12–128 байт): ")
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return errors.New("cannot read password")
		}
		fmt.Fprint(os.Stderr, "Повторите пароль: ")
		confirmation, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		defer clear(confirmation)
		if e != nil || string(password) != string(confirmation) {
			return errors.New("passwords do not match")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	result, err := demodata.Seed(ctx, pool, *date, string(password))
	if err != nil {
		// PostgreSQL details may contain row data. Never print them or connection URLs.
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			return fmt.Errorf("demo load rolled back (PostgreSQL %s); check schema and demo login/ID conflicts", pg.Code)
		}
		return errors.New("demo load failed; check database availability, migrations, date and password length (12–128 bytes)")
	}
	return enc.Encode(result)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
