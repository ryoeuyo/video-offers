package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env             string        `env:"APP_ENV" envDefault:"dev"`
	Addr            string        `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel        string        `env:"LOG_LEVEL" envDefault:"info"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	DB        DB
	Auth      Auth
	RateLimit RateLimit
}

type RateLimit struct {
	AuthMax        int           `env:"RATE_LIMIT_AUTH_MAX" envDefault:"30"`
	AuthWindow     time.Duration `env:"RATE_LIMIT_AUTH_WINDOW" envDefault:"1m"`
	OfferCreateMax int           `env:"RATE_LIMIT_OFFER_MAX" envDefault:"10"`
	OfferWindow    time.Duration `env:"RATE_LIMIT_OFFER_WINDOW" envDefault:"1m"`
}

type DB struct {
	DSN             string        `env:"DATABASE_URL,required"`
	MaxConns        int32         `env:"DB_MAX_CONNS" envDefault:"10"`
	MinConns        int32         `env:"DB_MIN_CONNS" envDefault:"2"`
	MaxConnLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	ConnectTimeout  time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"5s"`
}

type Auth struct {
	JWTSecret  string        `env:"JWT_SECRET,required"`
	AccessTTL  time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTTL time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
}

func (c Config) IsDev() bool { return c.Env == "dev" }

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	// Дефолтный секрет в проде — самая дешёвая дыра, ловим её на старте.
	if !c.IsDev() && len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes in %s", c.Env)
	}
	if c.DB.MinConns > c.DB.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS (%d) > DB_MAX_CONNS (%d)", c.DB.MinConns, c.DB.MaxConns)
	}
	return nil
}
