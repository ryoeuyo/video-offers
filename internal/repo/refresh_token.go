package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/domain"
)

type RefreshTokenRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepo(pool *pgxpool.Pool) *RefreshTokenRepo {
	return &RefreshTokenRepo{pool: pool}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, t domain.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, q, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return MapError(err)
}

func (r *RefreshTokenRepo) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`

	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt,
	)
	if err != nil {
		return domain.RefreshToken{}, MapError(err)
	}
	return t, nil
}

func (r *RefreshTokenRepo) MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	const q = `
		UPDATE refresh_tokens SET used_at = $2 WHERE id = $1 AND used_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, id, usedAt)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUnauthorized.WithCode("invalid_token", "refresh-токен уже использован или не найден")
	}
	return nil
}
