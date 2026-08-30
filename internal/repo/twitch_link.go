package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/domain"
)

type TwitchLinkRepo struct {
	pool *pgxpool.Pool
}

func NewTwitchLinkRepo(pool *pgxpool.Pool) *TwitchLinkRepo {
	return &TwitchLinkRepo{pool: pool}
}

func (r *TwitchLinkRepo) Upsert(ctx context.Context, rec domain.TwitchLinkRecord) error {
	const q = `
		INSERT INTO user_twitch_links (
			user_id, twitch_user_id, twitch_login, twitch_display_name,
			access_token_enc, refresh_token_enc, token_expires_at, scopes,
			linked_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO UPDATE SET
			twitch_user_id = EXCLUDED.twitch_user_id,
			twitch_login = EXCLUDED.twitch_login,
			twitch_display_name = EXCLUDED.twitch_display_name,
			access_token_enc = EXCLUDED.access_token_enc,
			refresh_token_enc = EXCLUDED.refresh_token_enc,
			token_expires_at = EXCLUDED.token_expires_at,
			scopes = EXCLUDED.scopes,
			updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, q,
		rec.UserID, rec.TwitchUserID, rec.TwitchLogin, rec.TwitchDisplayName,
		rec.AccessTokenEnc, rec.RefreshTokenEnc, rec.TokenExpiresAt, rec.Scopes,
		rec.LinkedAt, rec.UpdatedAt,
	)
	if err != nil {
		if IsUniqueViolation(err, "user_twitch_links_twitch_user_id_key") {
			return domain.ErrConflict.WithCode("twitch_already_linked", "этот Twitch уже привязан к другому аккаунту")
		}
		return MapError(err)
	}
	return nil
}

func (r *TwitchLinkRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.TwitchLink, error) {
	const q = `
		SELECT user_id, twitch_user_id, twitch_login, twitch_display_name,
		       scopes, token_expires_at, linked_at, updated_at
		FROM user_twitch_links WHERE user_id = $1`

	var link domain.TwitchLink
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&link.UserID, &link.TwitchUserID, &link.TwitchLogin, &link.TwitchDisplayName,
		&link.Scopes, &link.TokenExpiresAt, &link.LinkedAt, &link.UpdatedAt,
	)
	if err != nil {
		return domain.TwitchLink{}, MapError(err)
	}
	return link, nil
}

func (r *TwitchLinkRepo) ExistsByUserID(ctx context.Context, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM user_twitch_links WHERE user_id = $1)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&ok); err != nil {
		return false, MapError(err)
	}
	return ok, nil
}

func (r *TwitchLinkRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM user_twitch_links WHERE user_id = $1`, userID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound.WithCode("twitch_not_linked", "Twitch не привязан")
	}
	return nil
}

func (r *TwitchLinkRepo) GetRecordByUserID(ctx context.Context, userID uuid.UUID) (domain.TwitchLinkRecord, error) {
	const q = `
		SELECT user_id, twitch_user_id, twitch_login, twitch_display_name,
		       access_token_enc, refresh_token_enc, token_expires_at, scopes,
		       linked_at, updated_at
		FROM user_twitch_links WHERE user_id = $1`

	var rec domain.TwitchLinkRecord
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&rec.UserID, &rec.TwitchUserID, &rec.TwitchLogin, &rec.TwitchDisplayName,
		&rec.AccessTokenEnc, &rec.RefreshTokenEnc, &rec.TokenExpiresAt, &rec.Scopes,
		&rec.LinkedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return domain.TwitchLinkRecord{}, MapError(err)
	}
	return rec, nil
}

func (r *TwitchLinkRepo) TouchUpdated(ctx context.Context, userID uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE user_twitch_links SET updated_at = $2 WHERE user_id = $1`, userID, at)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
