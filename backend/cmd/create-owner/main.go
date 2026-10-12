package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

func run() error {
	login := flag.String("login", "", "owner login (no password in arguments)")
	name := flag.String("name", "", "owner display name")
	flag.Parse()
	if *login == "" || *name == "" {
		return errors.New("--login and --name are required")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("interactive terminal required for hidden password input")
	}
	cfg, err := platform.LoadConfig(false)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "Password (12–128 bytes): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return errors.New("cannot read password")
	}
	defer clear(password)
	fmt.Fprint(os.Stderr, "Repeat password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	defer clear(confirmation)
	if err != nil || string(password) != string(confirmation) {
		return errors.New("passwords do not match")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	return auth.CreateFirstOwner(ctx, pool, *login, *name, string(password))
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("First owner created.")
}
