package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/domain"
)

type StreamerSettingsRepo struct {
	pool *pgxpool.Pool
}

func NewStreamerSettingsRepo(pool *pgxpool.Pool) *StreamerSettingsRepo {
	return &StreamerSettingsRepo{pool: pool}
}

func (r *StreamerSettingsRepo) Create(ctx context.Context, s domain.StreamerSettings) error {
	const q = `
		INSERT INTO streamer_settings (user_id, accepting_offers, allow_anonymous, min_account_age_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	secs := int(s.MinAccountAge / time.Second)
	_, err := r.pool.Exec(ctx, q,
		s.UserID, s.AcceptingOffers, s.AllowAnonymous, secs, s.CreatedAt, s.UpdatedAt,
	)
	return MapError(err)
}

func (r *StreamerSettingsRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	const q = `
		SELECT user_id, accepting_offers, allow_anonymous, min_account_age_seconds, created_at, updated_at
		FROM streamer_settings WHERE user_id = $1`

	var s domain.StreamerSettings
	var secs int
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&s.UserID, &s.AcceptingOffers, &s.AllowAnonymous, &secs, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return domain.StreamerSettings{}, MapError(err)
	}
	s.MinAccountAge = time.Duration(secs) * time.Second
	return s, nil
}

func (r *StreamerSettingsRepo) Update(ctx context.Context, s domain.StreamerSettings) error {
	const q = `
		UPDATE streamer_settings
		SET accepting_offers = $2, allow_anonymous = $3, min_account_age_seconds = $4, updated_at = $5
		WHERE user_id = $1`

	secs := int(s.MinAccountAge / time.Second)
	tag, err := r.pool.Exec(ctx, q,
		s.UserID, s.AcceptingOffers, s.AllowAnonymous, secs, s.UpdatedAt,
	)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
