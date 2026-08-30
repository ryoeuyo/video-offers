package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/pagination"
	"github.com/ruslan/video-offers/internal/repo"
	"github.com/ruslan/video-offers/internal/service/video"
	pkguuid "github.com/ruslan/video-offers/internal/pkg/uuid"
)

type OfferRepository interface {
	Create(ctx context.Context, o domain.Offer) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Offer, error)
	ListByStreamer(ctx context.Context, p repo.ListStreamerOffersParams) ([]domain.Offer, error)
	ListBySender(ctx context.Context, p repo.ListSenderOffersParams) ([]domain.Offer, error)
	UpdateStatus(ctx context.Context, streamerID, offerID uuid.UUID, status domain.OfferStatus, watchedAt *time.Time) (domain.Offer, error)
	UpdateMeta(ctx context.Context, offerID uuid.UUID, title, thumbnailURL string) error
	DeleteByStreamer(ctx context.Context, streamerID, offerID uuid.UUID) error
	DeletePendingBySender(ctx context.Context, senderID, offerID uuid.UUID) error
}

type OfferStreamerLookup interface {
	GetByUsername(ctx context.Context, username string) (domain.User, error)
}

type OfferSettingsLookup interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error)
}

type CreateOfferInput struct {
	URL     string
	Comment string
}

type OfferService struct {
	offers   OfferRepository
	streamers OfferStreamerLookup
	settings OfferSettingsLookup
	resolver video.Resolver
}

func NewOfferService(
	offers OfferRepository,
	streamers OfferStreamerLookup,
	settings OfferSettingsLookup,
	resolver video.Resolver,
) *OfferService {
	return &OfferService{
		offers:    offers,
		streamers: streamers,
		settings:  settings,
		resolver:  resolver,
	}
}

func (s *OfferService) Create(
	ctx context.Context,
	streamerUsername string,
	senderID uuid.UUID,
	in CreateOfferInput,
) (domain.Offer, error) {
	streamer, err := s.streamers.GetByUsername(ctx, streamerUsername)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Offer{}, domain.ErrNotFound.WithCode("streamer_not_found", "стример не найден")
		}
		return domain.Offer{}, fmt.Errorf("get streamer: %w", err)
	}
	if streamer.Role != domain.RoleStreamer {
		return domain.Offer{}, domain.ErrNotFound.WithCode("streamer_not_found", "стример не найден")
	}

	settings, err := s.settings.GetByUserID(ctx, streamer.ID)
	if err != nil {
		return domain.Offer{}, fmt.Errorf("get streamer settings: %w", err)
	}
	if !settings.AcceptingOffers {
		return domain.Offer{}, domain.ErrConflict.WithCode("offers_disabled", "стример не принимает офферы")
	}

	comment := strings.TrimSpace(in.Comment)
	if utf8.RuneCountInString(comment) > 500 {
		return domain.Offer{}, domain.ErrInvalidInput.WithCode("validation_error", "comment максимум 500 символов").
			WithDetails(map[string]any{"field": "comment"})
	}

	parsed, err := video.ParseURL(in.URL)
	if err != nil {
		return domain.Offer{}, err
	}

	meta := domain.VideoMeta{
		Provider:   parsed.Provider,
		ExternalID: parsed.ExternalID,
	}
	if s.resolver != nil {
		resolved, resolveErr := s.resolver.Resolve(ctx, parsed)
		if resolveErr == nil {
			meta = resolved
		}
	}
	if meta.ThumbnailURL == "" && parsed.Provider == domain.ProviderYouTube {
		meta.ThumbnailURL = video.YouTubeThumbnailURL(parsed.ExternalID)
	}

	id, err := pkguuid.New()
	if err != nil {
		return domain.Offer{}, err
	}

	offer := domain.Offer{
		ID:              id,
		StreamerID:      streamer.ID,
		SenderID:        &senderID,
		URL:             parsed.Original,
		NormalizedURL:   parsed.Normalized,
		Provider:        meta.Provider,
		ExternalID:      meta.ExternalID,
		Title:           meta.Title,
		ThumbnailURL:    meta.ThumbnailURL,
		DurationSeconds: meta.DurationSeconds,
		Comment:         comment,
		Status:          domain.StatusPending,
		CreatedAt:       time.Now().UTC(),
	}

	if err := s.offers.Create(ctx, offer); err != nil {
		return domain.Offer{}, err
	}
	return offer, nil
}

func (s *OfferService) enrichOffersMeta(ctx context.Context, offers []domain.Offer) []domain.Offer {
	if s.resolver == nil {
		return offers
	}

	for i := range offers {
		o := &offers[i]
		if o.Title != "" || o.Provider != domain.ProviderYouTube || o.ExternalID == "" {
			continue
		}

		resolveCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		meta, err := s.resolver.Resolve(resolveCtx, video.ParsedURL{
			Original:   o.URL,
			Normalized: o.NormalizedURL,
			Provider:   o.Provider,
			ExternalID: o.ExternalID,
		})
		cancel()
		if err != nil || meta.Title == "" {
			continue
		}

		thumb := meta.ThumbnailURL
		if thumb == "" {
			thumb = video.YouTubeThumbnailURL(o.ExternalID)
		}
		if err := s.offers.UpdateMeta(ctx, o.ID, meta.Title, thumb); err != nil {
			continue
		}

		o.Title = meta.Title
		if o.ThumbnailURL == "" {
			o.ThumbnailURL = thumb
		}
	}

	return offers
}

type OfferListPage struct {
	Items      []domain.Offer
	NextCursor string
}

func (s *OfferService) ListQueue(
	ctx context.Context,
	streamerID uuid.UUID,
	statusRaw, cursorRaw string,
	limit int,
) (OfferListPage, error) {
	limit = pagination.ClampLimit(limit, 20, 100)

	var status *domain.OfferStatus
	if statusRaw != "" {
		st := domain.OfferStatus(statusRaw)
		if !st.Valid() {
			return OfferListPage{}, domain.ErrInvalidInput.WithCode("validation_error", "некорректный status").
				WithDetails(map[string]any{"field": "status"})
		}
		status = &st
	}

	var cursor *pagination.Key
	if cursorRaw != "" {
		key, err := pagination.Decode(cursorRaw)
		if err != nil {
			return OfferListPage{}, domain.ErrInvalidInput.WithCode("invalid_cursor", "некорректный cursor").Wrap(err)
		}
		cursor = &key
	}

	items, err := s.offers.ListByStreamer(ctx, repo.ListStreamerOffersParams{
		StreamerID: streamerID,
		Status:     status,
		Cursor:     cursor,
		Limit:      limit,
	})
	if err != nil {
		return OfferListPage{}, err
	}
	items = s.enrichOffersMeta(ctx, items)
	return paginateOffers(items, limit), nil
}

func (s *OfferService) ListSent(
	ctx context.Context,
	senderID uuid.UUID,
	cursorRaw string,
	limit int,
) (OfferListPage, error) {
	limit = pagination.ClampLimit(limit, 20, 100)

	var cursor *pagination.Key
	if cursorRaw != "" {
		key, err := pagination.Decode(cursorRaw)
		if err != nil {
			return OfferListPage{}, domain.ErrInvalidInput.WithCode("invalid_cursor", "некорректный cursor").Wrap(err)
		}
		cursor = &key
	}

	items, err := s.offers.ListBySender(ctx, repo.ListSenderOffersParams{
		SenderID: senderID,
		Cursor:   cursor,
		Limit:    limit,
	})
	if err != nil {
		return OfferListPage{}, err
	}
	items = s.enrichOffersMeta(ctx, items)
	return paginateOffers(items, limit), nil
}

func (s *OfferService) UpdateStatus(
	ctx context.Context,
	streamerID, offerID uuid.UUID,
	next domain.OfferStatus,
) (domain.Offer, error) {
	if !next.Valid() || next == domain.StatusPending {
		return domain.Offer{}, domain.ErrInvalidInput.WithCode("validation_error", "status должен быть watched, skipped или rejected").
			WithDetails(map[string]any{"field": "status"})
	}

	existing, err := s.offers.GetByID(ctx, offerID)
	if err != nil {
		return domain.Offer{}, err
	}
	if existing.StreamerID != streamerID {
		return domain.Offer{}, domain.ErrNotFound
	}
	if !existing.Status.CanTransitionTo(next) {
		return domain.Offer{}, domain.ErrConflict.WithCode("invalid_transition", "статус уже изменён")
	}

	var watchedAt *time.Time
	if next == domain.StatusWatched {
		now := time.Now().UTC()
		watchedAt = &now
	}

	return s.offers.UpdateStatus(ctx, streamerID, offerID, next, watchedAt)
}

func (s *OfferService) DeleteByStreamer(ctx context.Context, streamerID, offerID uuid.UUID) error {
	offer, err := s.offers.GetByID(ctx, offerID)
	if err != nil {
		return err
	}
	if offer.StreamerID != streamerID {
		return domain.ErrNotFound
	}
	return s.offers.DeleteByStreamer(ctx, streamerID, offerID)
}

func (s *OfferService) RevokeSent(ctx context.Context, senderID, offerID uuid.UUID) error {
	return s.offers.DeletePendingBySender(ctx, senderID, offerID)
}

func paginateOffers(items []domain.Offer, limit int) OfferListPage {
	page := OfferListPage{Items: make([]domain.Offer, 0, len(items))}
	for i, item := range items {
		if i >= limit {
			last := items[limit-1]
			page.NextCursor = pagination.Encode(pagination.Key{
				CreatedAt: last.CreatedAt,
				ID:        last.ID,
			})
			break
		}
		page.Items = append(page.Items, item)
	}
	return page
}
