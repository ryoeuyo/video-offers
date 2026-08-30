package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/pagination"
)

type OfferRepo struct {
	pool *pgxpool.Pool
}

func NewOfferRepo(pool *pgxpool.Pool) *OfferRepo {
	return &OfferRepo{pool: pool}
}

func (r *OfferRepo) Create(ctx context.Context, o domain.Offer) error {
	const q = `
		INSERT INTO offers (
			id, streamer_id, sender_id, url, normalized_url, provider, external_id,
			title, thumbnail_url, duration_seconds, comment, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := r.pool.Exec(ctx, q,
		o.ID, o.StreamerID, o.SenderID, o.URL, o.NormalizedURL, o.Provider, o.ExternalID,
		o.Title, o.ThumbnailURL, o.DurationSeconds, o.Comment, o.Status, o.CreatedAt,
	)
	if err != nil {
		return mapOfferCreateError(err)
	}
	return nil
}

func mapOfferCreateError(err error) error {
	if IsUniqueViolation(err, "offers_pending_dedup_key") {
		return domain.ErrConflict.WithCode("offer_duplicate", "видео уже в очереди")
	}
	return MapError(err)
}

func (r *OfferRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Offer, error) {
	const q = `
		SELECT id, streamer_id, sender_id, url, normalized_url, provider, external_id,
		       title, thumbnail_url, duration_seconds, comment, status, watched_at, created_at
		FROM offers WHERE id = $1`

	var o domain.Offer
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.StreamerID, &o.SenderID, &o.URL, &o.NormalizedURL, &o.Provider, &o.ExternalID,
		&o.Title, &o.ThumbnailURL, &o.DurationSeconds, &o.Comment, &o.Status, &o.WatchedAt, &o.CreatedAt,
	)
	if err != nil {
		return domain.Offer{}, MapError(err)
	}
	return o, nil
}

type ListStreamerOffersParams struct {
	StreamerID uuid.UUID
	Status     *domain.OfferStatus
	Cursor     *pagination.Key
	Limit      int
}

type ListSenderOffersParams struct {
	SenderID uuid.UUID
	Cursor   *pagination.Key
	Limit    int
}

func (r *OfferRepo) ListByStreamer(ctx context.Context, p ListStreamerOffersParams) ([]domain.Offer, error) {
	limit := p.Limit + 1
	args := []any{p.StreamerID}
	q := `
		SELECT id, streamer_id, sender_id, url, normalized_url, provider, external_id,
		       title, thumbnail_url, duration_seconds, comment, status, watched_at, created_at
		FROM offers
		WHERE streamer_id = $1`

	if p.Status != nil {
		args = append(args, *p.Status)
		q += fmt.Sprintf(` AND status = $%d`, len(args))
	}

	if p.Cursor != nil {
		args = append(args, p.Cursor.CreatedAt, p.Cursor.ID)
		q += fmt.Sprintf(` AND (created_at, id) < ($%d, $%d)`, len(args)-1, len(args))
	}

	args = append(args, limit)
	q += fmt.Sprintf(`
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, len(args))

	return r.queryOffers(ctx, q, args...)
}

func (r *OfferRepo) ListBySender(ctx context.Context, p ListSenderOffersParams) ([]domain.Offer, error) {
	limit := p.Limit + 1
	args := []any{p.SenderID}
	q := `
		SELECT id, streamer_id, sender_id, url, normalized_url, provider, external_id,
		       title, thumbnail_url, duration_seconds, comment, status, watched_at, created_at
		FROM offers
		WHERE sender_id = $1`

	if p.Cursor != nil {
		args = append(args, p.Cursor.CreatedAt, p.Cursor.ID)
		q += fmt.Sprintf(` AND (created_at, id) < ($%d, $%d)`, len(args)-1, len(args))
	}

	args = append(args, limit)
	q += fmt.Sprintf(`
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, len(args))

	return r.queryOffers(ctx, q, args...)
}

func (r *OfferRepo) queryOffers(ctx context.Context, q string, args ...any) ([]domain.Offer, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, MapError(err)
	}
	defer rows.Close()

	var offers []domain.Offer
	for rows.Next() {
		var o domain.Offer
		if err := rows.Scan(
			&o.ID, &o.StreamerID, &o.SenderID, &o.URL, &o.NormalizedURL, &o.Provider, &o.ExternalID,
			&o.Title, &o.ThumbnailURL, &o.DurationSeconds, &o.Comment, &o.Status, &o.WatchedAt, &o.CreatedAt,
		); err != nil {
			return nil, MapError(err)
		}
		offers = append(offers, o)
	}
	if err := rows.Err(); err != nil {
		return nil, MapError(err)
	}
	return offers, nil
}

func (r *OfferRepo) UpdateStatus(ctx context.Context, streamerID, offerID uuid.UUID, status domain.OfferStatus, watchedAt *time.Time) (domain.Offer, error) {
	const q = `
		UPDATE offers
		SET status = $3, watched_at = $4
		WHERE id = $1 AND streamer_id = $2 AND status = 'pending'
		RETURNING id, streamer_id, sender_id, url, normalized_url, provider, external_id,
		          title, thumbnail_url, duration_seconds, comment, status, watched_at, created_at`

	var o domain.Offer
	err := r.pool.QueryRow(ctx, q, offerID, streamerID, status, watchedAt).Scan(
		&o.ID, &o.StreamerID, &o.SenderID, &o.URL, &o.NormalizedURL, &o.Provider, &o.ExternalID,
		&o.Title, &o.ThumbnailURL, &o.DurationSeconds, &o.Comment, &o.Status, &o.WatchedAt, &o.CreatedAt,
	)
	if err != nil {
		mapped := MapError(err)
		if errors.Is(mapped, domain.ErrNotFound) {
			existing, getErr := r.GetByID(ctx, offerID)
			if getErr != nil {
				return domain.Offer{}, getErr
			}
			if existing.StreamerID != streamerID {
				return domain.Offer{}, domain.ErrNotFound
			}
			return domain.Offer{}, domain.ErrConflict.WithCode("invalid_transition", "статус уже изменён")
		}
		return domain.Offer{}, mapped
	}
	return o, nil
}

func (r *OfferRepo) UpdateMeta(ctx context.Context, offerID uuid.UUID, title, thumbnailURL string) error {
	const q = `
		UPDATE offers
		SET title = $2,
		    thumbnail_url = CASE WHEN thumbnail_url = '' THEN $3 ELSE thumbnail_url END
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, offerID, title, thumbnailURL)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *OfferRepo) DeleteByStreamer(ctx context.Context, streamerID, offerID uuid.UUID) error {
	const q = `DELETE FROM offers WHERE id = $1 AND streamer_id = $2`
	tag, err := r.pool.Exec(ctx, q, offerID, streamerID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *OfferRepo) DeletePendingBySender(ctx context.Context, senderID, offerID uuid.UUID) error {
	const q = `DELETE FROM offers WHERE id = $1 AND sender_id = $2 AND status = 'pending'`
	tag, err := r.pool.Exec(ctx, q, offerID, senderID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := r.GetByID(ctx, offerID)
		if err != nil {
			return err
		}
		if existing.SenderID == nil || *existing.SenderID != senderID {
			return domain.ErrNotFound
		}
		return domain.ErrConflict.WithCode("offer_not_pending", "можно отозвать только pending-оффер")
	}
	return nil
}
