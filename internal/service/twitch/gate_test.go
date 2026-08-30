package twitch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/seal"
)

type stubHelix struct {
	following  bool
	followedAt time.Time
	followErr  error
	sub        bool
	subErr     error
}

func (s *stubHelix) GetFollowedAt(context.Context, string, string, string) (time.Time, bool, error) {
	return s.followedAt, s.following, s.followErr
}

func (s *stubHelix) IsSubscribed(context.Context, string, string, string) (bool, error) {
	return s.sub, s.subErr
}

func sealedRecord(t *testing.T, userID uuid.UUID, twitchID, key string) domain.TwitchLinkRecord {
	t.Helper()
	access, err := seal.Seal(key, "access")
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := seal.Seal(key, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return domain.TwitchLinkRecord{
		TwitchLink: domain.TwitchLink{
			UserID:         userID,
			TwitchUserID:   twitchID,
			TwitchLogin:    "login",
			TokenExpiresAt: now.Add(time.Hour),
			LinkedAt:       now,
			UpdatedAt:      now,
		},
		AccessTokenEnc:  access,
		RefreshTokenEnc: refresh,
	}
}

func TestCheckOfferGates(t *testing.T) {
	key := "gate-test-key-32-bytes-minimum!!"
	streamerID := uuid.Must(uuid.NewV7())
	senderID := uuid.Must(uuid.NewV7())
	repo := &fakeLinkRepo{byUser: map[uuid.UUID]domain.TwitchLinkRecord{
		streamerID: sealedRecord(t, streamerID, "broadcaster", key),
		senderID:   sealedRecord(t, senderID, "viewer", key),
	}}
	svc := NewLinkService(repo, &fakeSettingsRepo{}, &fakeUserRepo{}, nil, key, "http://front", true)
	svc.helix = &stubHelix{following: true, followedAt: time.Now().Add(-48 * time.Hour), sub: true}

	settings := domain.StreamerSettings{RequireFollow: true, MinFollowAge: 24 * time.Hour, RequireSubscription: true}
	if err := svc.CheckOfferGates(context.Background(), streamerID, senderID, settings); err != nil {
		t.Fatalf("CheckOfferGates: %v", err)
	}

	svc.helix = &stubHelix{following: false}
	err := svc.CheckOfferGates(context.Background(), streamerID, senderID, domain.StreamerSettings{RequireFollow: true})
	if e, ok := domain.AsError(err); !ok || e.Code != "twitch_not_following" {
		t.Fatalf("got %v, want twitch_not_following", err)
	}

	svc.helix = &stubHelix{following: true, followedAt: time.Now()}
	err = svc.CheckOfferGates(context.Background(), streamerID, senderID, domain.StreamerSettings{RequireFollow: true, MinFollowAge: 24 * time.Hour})
	if e, ok := domain.AsError(err); !ok || e.Code != "twitch_follow_too_new" {
		t.Fatalf("got %v, want twitch_follow_too_new", err)
	}

	svc.helix = &stubHelix{sub: false}
	err = svc.CheckOfferGates(context.Background(), streamerID, senderID, domain.StreamerSettings{RequireSubscription: true})
	if e, ok := domain.AsError(err); !ok || e.Code != "twitch_subscription_required" {
		t.Fatalf("got %v, want twitch_subscription_required", err)
	}

	err = svc.CheckOfferGates(context.Background(), streamerID, uuid.Must(uuid.NewV7()), domain.StreamerSettings{RequireTwitchSender: true})
	if e, ok := domain.AsError(err); !ok || e.Code != "twitch_not_linked" {
		t.Fatalf("got %v, want twitch_not_linked", err)
	}

	svc.helix = &stubHelix{followErr: context.DeadlineExceeded}
	svc.gateCache = make(map[string]gateCacheEntry)
	err = svc.CheckOfferGates(context.Background(), streamerID, senderID, domain.StreamerSettings{RequireFollow: true})
	if e, ok := domain.AsError(err); !ok || e.Code != "twitch_unavailable" {
		t.Fatalf("got %v, want twitch_unavailable", err)
	}
}
