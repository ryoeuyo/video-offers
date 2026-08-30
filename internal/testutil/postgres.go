package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/repo"
)

// PostgresContainer поднимает Postgres 16, накатывает goose-миграции и возвращает пул.
func PostgresContainer(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("video_offers"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("connection string: %v", err)
	}

	if err := migrateUp(t, dsn); err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("migrate: %v", err)
	}

	pool, err := repo.NewPool(ctx, config.DB{
		DSN:            dsn,
		MaxConns:       5,
		MinConns:       1,
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("connect pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_ = container.Terminate(context.Background())
	}
	return pool, cleanup
}

func migrateUp(t *testing.T, dsn string) error {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("runtime caller")
	}
	migrationsDir := filepath.Join(filepath.Dir(file), "..", "..", "migrations")

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
