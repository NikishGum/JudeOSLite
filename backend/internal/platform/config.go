package platform

import (
	"errors"
	"net"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	StaticDir   string
}

func LoadConfig(server bool) (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), HTTPAddr: os.Getenv("HTTP_ADDR"), StaticDir: os.Getenv("STATIC_DIR")}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if _, err := pgxpool.ParseConfig(c.DatabaseURL); err != nil {
		return c, errors.New("DATABASE_URL is invalid")
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	if c.StaticDir == "" {
		c.StaticDir = "../frontend/dist"
	}
	if server {
		_, port, err := net.SplitHostPort(c.HTTPAddr)
		p, parseErr := strconv.Atoi(port)
		if err != nil || parseErr != nil || p < 1 || p > 65535 {
			return c, errors.New("HTTP_ADDR must be host:port with a valid port")
		}
	}
	return c, nil
}
