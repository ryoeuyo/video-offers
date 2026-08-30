package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/domain"
)

// NewPool поднимает пул и сразу пингует БД: падать лучше на старте,
// чем на первом запросе пользователя.
func NewPool(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// Коды ошибок PostgreSQL, которые для нас имеют доменный смысл.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// MapError превращает ошибки драйвера в доменные. Вызывать в каждом методе
// репозитория — выше по стеку про pgx знать не должны.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound.Wrap(err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return domain.ErrConflict.Wrap(err)
		case pgForeignKeyViolation:
			return domain.ErrInvalidInput.Wrap(err)
		}
	}
	return err
}

// IsUniqueViolation нужен там, где важно отличить конкретный индекс,
// например дубль оффера в очереди от занятого username.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == constraint
}
