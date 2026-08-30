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
		INSERT INTO streamer_settings (
			user_id, accepting_offers, allow_anonymous, min_account_age_seconds,
			require_twitch_sender, require_follow, min_follow_age_seconds, require_subscription,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, q,
		s.UserID, s.AcceptingOffers, s.AllowAnonymous, durationSeconds(s.MinAccountAge),
		s.RequireTwitchSender, s.RequireFollow, durationSeconds(s.MinFollowAge), s.RequireSubscription,
		s.CreatedAt, s.UpdatedAt,
	)
	return MapError(err)
}

func (r *StreamerSettingsRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	const q = `
		SELECT user_id, accepting_offers, allow_anonymous, min_account_age_seconds,
		       require_twitch_sender, require_follow, min_follow_age_seconds, require_subscription,
		       created_at, updated_at
		FROM streamer_settings WHERE user_id = $1`

	var s domain.StreamerSettings
	var minAccount, minFollow int
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&s.UserID, &s.AcceptingOffers, &s.AllowAnonymous, &minAccount,
		&s.RequireTwitchSender, &s.RequireFollow, &minFollow, &s.RequireSubscription,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return domain.StreamerSettings{}, MapError(err)
	}
	s.MinAccountAge = time.Duration(minAccount) * time.Second
	s.MinFollowAge = time.Duration(minFollow) * time.Second
	return s, nil
}

func (r *StreamerSettingsRepo) Update(ctx context.Context, s domain.StreamerSettings) error {
	const q = `
		UPDATE streamer_settings
		SET accepting_offers = $2, allow_anonymous = $3, min_account_age_seconds = $4,
		    require_twitch_sender = $5, require_follow = $6, min_follow_age_seconds = $7,
		    require_subscription = $8, updated_at = $9
		WHERE user_id = $1`

	tag, err := r.pool.Exec(ctx, q,
		s.UserID, s.AcceptingOffers, s.AllowAnonymous, durationSeconds(s.MinAccountAge),
		s.RequireTwitchSender, s.RequireFollow, durationSeconds(s.MinFollowAge), s.RequireSubscription,
		s.UpdatedAt,
	)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func durationSeconds(d time.Duration) int {
	return int(saneNonNegative(d) / time.Second)
}

func saneNonNegative(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}
