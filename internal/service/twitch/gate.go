package twitch

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/seal"
)

type gateCacheEntry struct {
	err error
	exp time.Time
}

func (s *LinkService) CheckOfferGates(
	ctx context.Context,
	streamerID, senderID uuid.UUID,
	settings domain.StreamerSettings,
) error {
	if !settings.TwitchGatesEnabled() {
		return nil
	}
	if !s.enabled || s.helix == nil {
		return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен")
	}

	cacheKey := fmt.Sprintf("%s:%s:%t:%t:%d:%t",
		streamerID, senderID, settings.RequireTwitchSender, settings.RequireFollow,
		settings.MinFollowAge, settings.RequireSubscription)
	if cached, ok := s.lookupGate(cacheKey); ok {
		return cached
	}

	err := s.checkOfferGatesUncached(ctx, streamerID, senderID, settings)
	if err == nil || isForbiddenGate(err) {
		s.storeGate(cacheKey, err)
	}
	return err
}

func (s *LinkService) checkOfferGatesUncached(
	ctx context.Context,
	streamerID, senderID uuid.UUID,
	settings domain.StreamerSettings,
) error {
	if !s.enabled || s.helix == nil {
		return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен")
	}

	senderRec, err := s.repo.GetRecordByUserID(ctx, senderID)
	if err != nil {
		if isNotFound(err) {
			return domain.ErrForbidden.WithCode("twitch_not_linked", "привяжите Twitch, чтобы отправлять офферы")
		}
		return fmt.Errorf("get sender twitch: %w", err)
	}

	if !settings.RequireFollow && !settings.RequireSubscription && settings.MinFollowAge == 0 {
		return nil
	}

	streamerRec, err := s.repo.GetRecordByUserID(ctx, streamerID)
	if err != nil {
		return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен").Wrap(err)
	}

	if settings.RequireFollow || settings.MinFollowAge > 0 {
		senderToken, err := s.accessToken(ctx, senderRec)
		if err != nil {
			return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен").Wrap(err)
		}
		followedAt, following, err := s.helix.GetFollowedAt(ctx, senderToken, streamerRec.TwitchUserID, senderRec.TwitchUserID)
		if err != nil {
			return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен").Wrap(err)
		}
		if !following {
			return domain.ErrForbidden.WithCode("twitch_not_following", "нужно быть фолловером канала стримера")
		}
		if settings.MinFollowAge > 0 && time.Since(followedAt) < settings.MinFollowAge {
			return domain.ErrForbidden.WithCode("twitch_follow_too_new", "фоллов слишком свежий для этой предложки")
		}
	}

	if settings.RequireSubscription {
		streamerToken, err := s.accessToken(ctx, streamerRec)
		if err != nil {
			return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен").Wrap(err)
		}
		ok, err := s.helix.IsSubscribed(ctx, streamerToken, streamerRec.TwitchUserID, senderRec.TwitchUserID)
		if err != nil {
			return domain.ErrUnavailable.WithCode("twitch_unavailable", "Twitch временно недоступен").Wrap(err)
		}
		if !ok {
			return domain.ErrForbidden.WithCode("twitch_subscription_required", "нужна подписка на канал стримера")
		}
	}

	return nil
}

func (s *LinkService) accessToken(ctx context.Context, rec domain.TwitchLinkRecord) (string, error) {
	if rec.TokenExpiresAt.After(time.Now().Add(30 * time.Second)) {
		return seal.Open(s.sealKey, rec.AccessTokenEnc)
	}
	if s.oauth == nil {
		return "", fmt.Errorf("oauth client missing")
	}
	refresh, err := seal.Open(s.sealKey, rec.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	tokens, err := s.oauth.Refresh(ctx, refresh)
	if err != nil {
		return "", fmt.Errorf("refresh twitch token: %w", err)
	}
	accessEnc, err := seal.Seal(s.sealKey, tokens.AccessToken)
	if err != nil {
		return "", err
	}
	refreshEnc := rec.RefreshTokenEnc
	if tokens.RefreshToken != "" {
		refreshEnc, err = seal.Seal(s.sealKey, tokens.RefreshToken)
		if err != nil {
			return "", err
		}
	}
	now := time.Now().UTC()
	rec.AccessTokenEnc = accessEnc
	rec.RefreshTokenEnc = refreshEnc
	rec.TokenExpiresAt = now.Add(time.Duration(tokens.ExpiresIn) * time.Second)
	rec.UpdatedAt = now
	if len(tokens.Scope) > 0 {
		rec.Scopes = tokens.Scope
	}
	if err := s.repo.Upsert(ctx, rec); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (s *LinkService) lookupGate(key string) (error, bool) {
	if s.gateCache == nil {
		return nil, false
	}
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	e, ok := s.gateCache[key]
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.err, true
}

func (s *LinkService) storeGate(key string, err error) {
	if s.gateCache == nil {
		return
	}
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	s.gateCache[key] = gateCacheEntry{err: err, exp: time.Now().Add(90 * time.Second)}
}

func isForbiddenGate(err error) bool {
	e, ok := domain.AsError(err)
	return ok && e.Kind == domain.KindForbidden
}
