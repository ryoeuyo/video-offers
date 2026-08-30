package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/repo"
	"github.com/ruslan/video-offers/internal/service/video"
)

type fakeOfferRepo struct {
	offers []domain.Offer
}

func (f *fakeOfferRepo) GetByID(_ context.Context, id uuid.UUID) (domain.Offer, error) {
	for _, o := range f.offers {
		if o.ID == id {
			return o, nil
		}
	}
	return domain.Offer{}, domain.ErrNotFound
}

func (f *fakeOfferRepo) ListByStreamer(_ context.Context, p repo.ListStreamerOffersParams) ([]domain.Offer, error) {
	var out []domain.Offer
	for _, o := range f.offers {
		if o.StreamerID != p.StreamerID {
			continue
		}
		if p.Status != nil && o.Status != *p.Status {
			continue
		}
		if p.Cursor != nil {
			if o.CreatedAt.After(p.Cursor.CreatedAt) {
				continue
			}
			if o.CreatedAt.Equal(p.Cursor.CreatedAt) && o.ID.String() >= p.Cursor.ID.String() {
				continue
			}
		}
		out = append(out, o)
	}
	if len(out) > p.Limit+1 {
		out = out[:p.Limit+1]
	}
	return out, nil
}

func (f *fakeOfferRepo) ListBySender(_ context.Context, p repo.ListSenderOffersParams) ([]domain.Offer, error) {
	var out []domain.Offer
	for _, o := range f.offers {
		if o.SenderID == nil || *o.SenderID != p.SenderID {
			continue
		}
		if p.Cursor != nil {
			if o.CreatedAt.After(p.Cursor.CreatedAt) {
				continue
			}
			if o.CreatedAt.Equal(p.Cursor.CreatedAt) && o.ID.String() >= p.Cursor.ID.String() {
				continue
			}
		}
		out = append(out, o)
	}
	if len(out) > p.Limit+1 {
		out = out[:p.Limit+1]
	}
	return out, nil
}

func (f *fakeOfferRepo) UpdateStatus(_ context.Context, streamerID, offerID uuid.UUID, status domain.OfferStatus, watchedAt *time.Time) (domain.Offer, error) {
	for i, o := range f.offers {
		if o.ID != offerID {
			continue
		}
		if o.StreamerID != streamerID {
			return domain.Offer{}, domain.ErrNotFound
		}
		if o.Status != domain.StatusPending {
			return domain.Offer{}, domain.ErrConflict.WithCode("invalid_transition", "статус уже изменён")
		}
		o.Status = status
		o.WatchedAt = watchedAt
		f.offers[i] = o
		return o, nil
	}
	return domain.Offer{}, domain.ErrNotFound
}

func (f *fakeOfferRepo) DeleteByStreamer(_ context.Context, streamerID, offerID uuid.UUID) error {
	for i, o := range f.offers {
		if o.ID == offerID && o.StreamerID == streamerID {
			f.offers = append(f.offers[:i], f.offers[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeOfferRepo) DeletePendingBySender(_ context.Context, senderID, offerID uuid.UUID) error {
	for i, o := range f.offers {
		if o.ID != offerID {
			continue
		}
		if o.SenderID == nil || *o.SenderID != senderID {
			return domain.ErrNotFound
		}
		if o.Status != domain.StatusPending {
			return domain.ErrConflict.WithCode("offer_not_pending", "можно отозвать только pending-оффер")
		}
		f.offers = append(f.offers[:i], f.offers[i+1:]...)
		return nil
	}
	return domain.ErrNotFound
}

func (f *fakeOfferRepo) Create(_ context.Context, o domain.Offer) error {
	for _, existing := range f.offers {
		if existing.StreamerID == o.StreamerID && existing.NormalizedURL == o.NormalizedURL && existing.Status == domain.StatusPending {
			return domain.ErrConflict.WithCode("offer_duplicate", "видео уже в очереди")
		}
	}
	f.offers = append(f.offers, o)
	return nil
}

type fakeOfferStreamerRepo struct {
	users map[string]domain.User
}

func (f *fakeOfferStreamerRepo) GetByUsername(_ context.Context, username string) (domain.User, error) {
	u, ok := f.users[username]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

type fakeOfferSettingsRepo struct {
	settings map[uuid.UUID]domain.StreamerSettings
}

func (f *fakeOfferSettingsRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	s, ok := f.settings[userID]
	if !ok {
		return domain.StreamerSettings{}, domain.ErrNotFound
	}
	return s, nil
}

type fakeVideoResolver struct {
	meta domain.VideoMeta
	err  error
}

func (f *fakeVideoResolver) Resolve(_ context.Context, p video.ParsedURL) (domain.VideoMeta, error) {
	if f.err != nil {
		return domain.VideoMeta{Provider: p.Provider, ExternalID: p.ExternalID}, f.err
	}
	meta := f.meta
	if meta.Provider == "" {
		meta.Provider = p.Provider
	}
	if meta.ExternalID == "" {
		meta.ExternalID = p.ExternalID
	}
	return meta, nil
}

func TestOfferService_Create_Success(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	senderID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	streamers := &fakeOfferStreamerRepo{users: map[string]domain.User{
		"alice": {ID: streamerID, Username: "alice", Role: domain.RoleStreamer},
	}}
	settings := &fakeOfferSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{
		streamerID: {UserID: streamerID, AcceptingOffers: true, CreatedAt: now, UpdatedAt: now},
	}}
	offers := &fakeOfferRepo{}
	resolver := &fakeVideoResolver{meta: domain.VideoMeta{
		Title: "Cool Video", ThumbnailURL: "https://img.example/thumb.jpg",
	}}

	svc := NewOfferService(offers, streamers, settings, resolver)
	offer, err := svc.Create(context.Background(), "alice", senderID, CreateOfferInput{
		URL:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Comment: "check this",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if offer.Title != "Cool Video" {
		t.Errorf("title = %q", offer.Title)
	}
	if offer.NormalizedURL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("normalized = %q", offer.NormalizedURL)
	}
	if offer.SenderID == nil || *offer.SenderID != senderID {
		t.Error("sender mismatch")
	}
}

func TestOfferService_Create_Duplicate(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	senderID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	streamers := &fakeOfferStreamerRepo{users: map[string]domain.User{
		"alice": {ID: streamerID, Username: "alice", Role: domain.RoleStreamer},
	}}
	settings := &fakeOfferSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{
		streamerID: {UserID: streamerID, AcceptingOffers: true, CreatedAt: now, UpdatedAt: now},
	}}
	offers := &fakeOfferRepo{}
	svc := NewOfferService(offers, streamers, settings, &fakeVideoResolver{})

	url := "https://youtu.be/dQw4w9WgXcQ"
	if _, err := svc.Create(context.Background(), "alice", senderID, CreateOfferInput{URL: url}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), "alice", senderID, CreateOfferInput{
		URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	assertDomainCode(t, err, "offer_duplicate")
}

func TestOfferService_Create_OffersDisabled(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	streamers := &fakeOfferStreamerRepo{users: map[string]domain.User{
		"alice": {ID: streamerID, Username: "alice", Role: domain.RoleStreamer},
	}}
	settings := &fakeOfferSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{
		streamerID: {UserID: streamerID, AcceptingOffers: false, CreatedAt: now, UpdatedAt: now},
	}}
	svc := NewOfferService(&fakeOfferRepo{}, streamers, settings, &fakeVideoResolver{})

	_, err := svc.Create(context.Background(), "alice", uuid.Must(uuid.NewV7()), CreateOfferInput{
		URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	assertDomainCode(t, err, "offers_disabled")
}

func TestOfferService_Create_ResolverFailureStillCreates(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	streamers := &fakeOfferStreamerRepo{users: map[string]domain.User{
		"alice": {ID: streamerID, Username: "alice", Role: domain.RoleStreamer},
	}}
	settings := &fakeOfferSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{
		streamerID: {UserID: streamerID, AcceptingOffers: true, CreatedAt: now, UpdatedAt: now},
	}}
	svc := NewOfferService(&fakeOfferRepo{}, streamers, settings, &fakeVideoResolver{err: context.DeadlineExceeded})

	offer, err := svc.Create(context.Background(), "alice", uuid.Must(uuid.NewV7()), CreateOfferInput{
		URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if offer.Title != "" {
		t.Errorf("expected empty title on resolver failure, got %q", offer.Title)
	}
}

func TestOfferService_Create_StreamerNotFound(t *testing.T) {
	svc := NewOfferService(&fakeOfferRepo{}, &fakeOfferStreamerRepo{users: map[string]domain.User{}}, &fakeOfferSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{}}, nil)
	_, err := svc.Create(context.Background(), "ghost", uuid.Must(uuid.NewV7()), CreateOfferInput{
		URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	assertDomainCode(t, err, "streamer_not_found")
}

func TestOfferService_UpdateStatus(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	offerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{{
		ID: offerID, StreamerID: streamerID, Status: domain.StatusPending,
		CreatedAt: time.Now().UTC(),
	}}}
	svc := NewOfferService(offers, nil, nil, nil)

	got, err := svc.UpdateStatus(context.Background(), streamerID, offerID, domain.StatusWatched)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if got.Status != domain.StatusWatched || got.WatchedAt == nil {
		t.Errorf("unexpected offer: %+v", got)
	}

	_, err = svc.UpdateStatus(context.Background(), streamerID, offerID, domain.StatusSkipped)
	assertDomainCode(t, err, "invalid_transition")
}

func TestOfferService_UpdateStatus_Forbidden(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	offerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{{
		ID: offerID, StreamerID: streamerID, Status: domain.StatusPending,
		CreatedAt: time.Now().UTC(),
	}}}
	svc := NewOfferService(offers, nil, nil, nil)

	_, err := svc.UpdateStatus(context.Background(), otherID, offerID, domain.StatusWatched)
	assertErrorKind(t, err, domain.KindNotFound)
}

func TestOfferService_ListQueue_FilterStatus(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{
		{ID: uuid.Must(uuid.NewV7()), StreamerID: streamerID, Status: domain.StatusPending, CreatedAt: time.Now().UTC()},
		{ID: uuid.Must(uuid.NewV7()), StreamerID: streamerID, Status: domain.StatusWatched, CreatedAt: time.Now().UTC()},
	}}
	svc := NewOfferService(offers, nil, nil, nil)

	page, err := svc.ListQueue(context.Background(), streamerID, "pending", "", 20)
	if err != nil {
		t.Fatalf("ListQueue: %v", err)
	}
	if len(page.Items) != 1 {
		t.Errorf("items = %d, want 1", len(page.Items))
	}
}

func TestOfferService_RevokeSent(t *testing.T) {
	senderID := uuid.Must(uuid.NewV7())
	offerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{{
		ID: offerID, SenderID: &senderID, Status: domain.StatusPending, CreatedAt: time.Now().UTC(),
	}}}
	svc := NewOfferService(offers, nil, nil, nil)

	if err := svc.RevokeSent(context.Background(), senderID, offerID); err != nil {
		t.Fatalf("RevokeSent: %v", err)
	}
	if len(offers.offers) != 0 {
		t.Error("offer should be deleted")
	}
}

func TestOfferService_RevokeSent_NotPending(t *testing.T) {
	senderID := uuid.Must(uuid.NewV7())
	offerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{{
		ID: offerID, SenderID: &senderID, Status: domain.StatusWatched, CreatedAt: time.Now().UTC(),
	}}}
	svc := NewOfferService(offers, nil, nil, nil)

	err := svc.RevokeSent(context.Background(), senderID, offerID)
	assertDomainCode(t, err, "offer_not_pending")
}

func TestOfferService_DeleteByStreamer(t *testing.T) {
	streamerID := uuid.Must(uuid.NewV7())
	offerID := uuid.Must(uuid.NewV7())
	offers := &fakeOfferRepo{offers: []domain.Offer{{
		ID: offerID, StreamerID: streamerID, Status: domain.StatusWatched, CreatedAt: time.Now().UTC(),
	}}}
	svc := NewOfferService(offers, nil, nil, nil)

	if err := svc.DeleteByStreamer(context.Background(), streamerID, offerID); err != nil {
		t.Fatalf("DeleteByStreamer: %v", err)
	}
	if len(offers.offers) != 0 {
		t.Error("offer should be deleted")
	}
}
