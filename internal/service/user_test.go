package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/pagination"
	"github.com/ruslan/video-offers/internal/repo"
)

type fakeProfileUserRepo struct {
	byID       map[uuid.UUID]domain.User
	byUsername map[string]domain.User
	streamers  []repo.StreamerListItem
}

func newFakeProfileUserRepo() *fakeProfileUserRepo {
	return &fakeProfileUserRepo{
		byID:       make(map[uuid.UUID]domain.User),
		byUsername: make(map[string]domain.User),
	}
}

func (f *fakeProfileUserRepo) seed(u domain.User) {
	f.byID[u.ID] = u
	f.byUsername[strings.ToLower(u.Username)] = u
}

func (f *fakeProfileUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeProfileUserRepo) GetByUsername(_ context.Context, username string) (domain.User, error) {
	u, ok := f.byUsername[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeProfileUserRepo) Update(_ context.Context, u domain.User) error {
	if _, ok := f.byID[u.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[u.ID] = u
	f.byUsername[strings.ToLower(u.Username)] = u
	return nil
}

func (f *fakeProfileUserRepo) ListStreamers(_ context.Context, p repo.ListStreamersParams) ([]repo.StreamerListItem, error) {
	var out []repo.StreamerListItem
	for _, item := range f.streamers {
		if p.UsernamePrefix != "" && !strings.HasPrefix(item.User.Username, strings.ToLower(p.UsernamePrefix)) {
			continue
		}
		if p.Cursor != nil {
			if item.User.CreatedAt.After(p.Cursor.CreatedAt) {
				continue
			}
			if item.User.CreatedAt.Equal(p.Cursor.CreatedAt) && item.User.ID.String() >= p.Cursor.ID.String() {
				continue
			}
		}
		out = append(out, item)
	}
	if len(out) > p.Limit+1 {
		out = out[:p.Limit+1]
	}
	return out, nil
}

type fakeSettingsRepo struct {
	byUser map[uuid.UUID]domain.StreamerSettings
}

func newFakeSettingsRepo() *fakeSettingsRepo {
	return &fakeSettingsRepo{byUser: make(map[uuid.UUID]domain.StreamerSettings)}
}

func (f *fakeSettingsRepo) Create(_ context.Context, s domain.StreamerSettings) error {
	f.byUser[s.UserID] = s
	return nil
}

func (f *fakeSettingsRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	s, ok := f.byUser[userID]
	if !ok {
		return domain.StreamerSettings{}, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeSettingsRepo) Update(_ context.Context, s domain.StreamerSettings) error {
	if _, ok := f.byUser[s.UserID]; !ok {
		return domain.ErrNotFound
	}
	f.byUser[s.UserID] = s
	return nil
}

func testUser(role domain.Role) domain.User {
	id := uuid.Must(uuid.NewV7())
	return domain.User{
		ID: id, Email: "user@example.com", Username: "testuser",
		Role: role, DisplayName: "Test User", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
}

func TestUpdateProfile_BecomeStreamerCreatesSettings(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	user := testUser(domain.RoleViewer)
	users.seed(user)

	role := domain.RoleStreamer
	updated, err := svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{Role: &role})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.Role != domain.RoleStreamer {
		t.Errorf("role = %q, want streamer", updated.Role)
	}

	s, ok := settings.byUser[user.ID]
	if !ok {
		t.Fatal("settings not created")
	}
	if !s.AcceptingOffers {
		t.Error("expected accepting_offers=true by default")
	}
}

func TestUpdateProfile_DisplayNameAndAvatar(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	user := testUser(domain.RoleViewer)
	users.seed(user)

	name := "New Name"
	avatar := "https://example.com/a.png"
	updated, err := svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{
		DisplayName: &name,
		AvatarURL:   &avatar,
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.DisplayName != name || updated.AvatarURL != avatar {
		t.Errorf("unexpected profile: %+v", updated)
	}
}

func TestUpdateProfile_ValidationErrors(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	user := testUser(domain.RoleViewer)
	users.seed(user)

	empty := ""
	_, err := svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{})
	assertDomainCode(t, err, "validation_error")

	_, err = svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{DisplayName: &empty})
	assertDomainCode(t, err, "validation_error")

	badAvatar := "ftp://bad"
	_, err = svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{AvatarURL: &badAvatar})
	assertDomainCode(t, err, "validation_error")

	badRole := domain.Role("admin")
	_, err = svc.UpdateProfile(ctx, user.ID, UpdateProfileInput{Role: &badRole})
	assertDomainCode(t, err, "validation_error")
}

func TestUpdateSettings_AcceptingOffers(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	user := testUser(domain.RoleStreamer)
	users.seed(user)
	now := time.Now().UTC()
	settings.byUser[user.ID] = domain.StreamerSettings{
		UserID: user.ID, AcceptingOffers: true, CreatedAt: now, UpdatedAt: now,
	}

	off := false
	got, err := svc.UpdateSettings(ctx, user.ID, UpdateSettingsInput{AcceptingOffers: &off})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if got.AcceptingOffers {
		t.Error("expected accepting_offers=false")
	}
}

func TestUpdateSettings_RequiresStreamer(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	user := testUser(domain.RoleViewer)
	users.seed(user)

	off := false
	_, err := svc.UpdateSettings(ctx, user.ID, UpdateSettingsInput{AcceptingOffers: &off})
	assertDomainCode(t, err, "streamer_required")
	assertErrorKind(t, err, domain.KindForbidden)
}

func TestGetPublicStreamer(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	streamer := testUser(domain.RoleStreamer)
	streamer.Username = "xqc"
	users.seed(streamer)
	now := time.Now().UTC()
	settings.byUser[streamer.ID] = domain.StreamerSettings{
		UserID: streamer.ID, AcceptingOffers: false, CreatedAt: now, UpdatedAt: now,
	}

	got, err := svc.GetPublicStreamer(ctx, "xqc")
	if err != nil {
		t.Fatalf("GetPublicStreamer: %v", err)
	}
	if got.Username != "xqc" || got.AcceptingOffers {
		t.Errorf("unexpected profile: %+v", got)
	}

	viewer := testUser(domain.RoleViewer)
	viewer.Username = "viewer1"
	users.seed(viewer)

	_, err = svc.GetPublicStreamer(ctx, "viewer1")
	assertDomainCode(t, err, "streamer_not_found")
}

func TestListStreamers_Pagination(t *testing.T) {
	users := newFakeProfileUserRepo()
	settings := newFakeSettingsRepo()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// streamers sorted DESC by created_at in real repo; seed in reverse for slice order
	for i := 2; i >= 0; i-- {
		u := testUser(domain.RoleStreamer)
		u.Username = "s" + string(rune('a'+i))
		u.CreatedAt = base.Add(time.Duration(i) * time.Hour)
		users.seed(u)
		users.streamers = append(users.streamers, repo.StreamerListItem{
			User: u, AcceptingOffers: true,
		})
	}

	page, err := svc.ListStreamers(ctx, "", "", 2)
	if err != nil {
		t.Fatalf("ListStreamers: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Fatal("expected next_cursor")
	}

	page2, err := svc.ListStreamers(ctx, "", page.NextCursor, 2)
	if err != nil {
		t.Fatalf("ListStreamers page2: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Errorf("page2 items = %d, want 1", len(page2.Items))
	}
}

func TestListStreamers_InvalidCursor(t *testing.T) {
	svc := NewUserService(newFakeProfileUserRepo(), newFakeSettingsRepo())
	_, err := svc.ListStreamers(context.Background(), "", "bad-cursor", 20)
	assertDomainCode(t, err, "invalid_cursor")
}

func TestPaginationEncodeDecodeUsedInList(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	at := time.Now().UTC()
	raw := pagination.Encode(pagination.Key{CreatedAt: at, ID: id})
	got, err := pagination.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Errorf("id mismatch")
	}
}
