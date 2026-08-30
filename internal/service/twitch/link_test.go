package twitch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
)

type fakeLinkRepo struct {
	byUser map[uuid.UUID]domain.TwitchLinkRecord
}

func (f *fakeLinkRepo) Upsert(_ context.Context, rec domain.TwitchLinkRecord) error {
	if f.byUser == nil {
		f.byUser = make(map[uuid.UUID]domain.TwitchLinkRecord)
	}
	for _, existing := range f.byUser {
		if existing.TwitchUserID == rec.TwitchUserID && existing.UserID != rec.UserID {
			return domain.ErrConflict.WithCode("twitch_already_linked", "already linked")
		}
	}
	f.byUser[rec.UserID] = rec
	return nil
}

func (f *fakeLinkRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.TwitchLink, error) {
	rec, ok := f.byUser[userID]
	if !ok {
		return domain.TwitchLink{}, domain.ErrNotFound
	}
	return rec.TwitchLink, nil
}

func (f *fakeLinkRepo) ExistsByUserID(_ context.Context, userID uuid.UUID) (bool, error) {
	_, ok := f.byUser[userID]
	return ok, nil
}

func (f *fakeLinkRepo) GetRecordByUserID(_ context.Context, userID uuid.UUID) (domain.TwitchLinkRecord, error) {
	rec, ok := f.byUser[userID]
	if !ok {
		return domain.TwitchLinkRecord{}, domain.ErrNotFound
	}
	return rec, nil
}

func (f *fakeLinkRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	if _, ok := f.byUser[userID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byUser, userID)
	return nil
}

type fakeSettingsRepo struct {
	settings map[uuid.UUID]domain.StreamerSettings
}

func (f *fakeSettingsRepo) GetByUserID(_ context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	s, ok := f.settings[userID]
	if !ok {
		return domain.StreamerSettings{}, domain.ErrNotFound
	}
	return s, nil
}

type fakeUserRepo struct {
	users map[uuid.UUID]domain.User
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func TestLinkService_OAuthFlow(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access123",
			"refresh_token": "refresh123",
			"expires_in":    3600,
			"scope":         []string{"user:read:follows"},
			"token_type":    "bearer",
		})
	}))
	defer tokenSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{
				"id": "999", "login": "coolstreamer", "display_name": "Cool Streamer",
			}},
		})
	}))
	defer apiSrv.Close()

	oauth := NewOAuthClient("client", "secret", "http://localhost/cb", tokenSrv.Client())
	oauth.tokenURL = tokenSrv.URL
	oauth.apiURL = apiSrv.URL

	svc := NewLinkService(
		&fakeLinkRepo{byUser: map[uuid.UUID]domain.TwitchLinkRecord{}},
		&fakeSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{}},
		&fakeUserRepo{users: map[uuid.UUID]domain.User{userID: {ID: userID, Role: domain.RoleViewer}}},
		oauth,
		"test-seal-key",
		"http://frontend/settings",
		true,
	)

	connectURL, err := svc.ConnectURL(context.Background(), userID)
	if err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	u, err := url.Parse(connectURL)
	if err != nil {
		t.Fatalf("parse connect url: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("expected state in connect url")
	}

	redirect, err := svc.HandleCallback(context.Background(), "auth-code", state)
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if !strings.Contains(redirect, "twitch=linked") {
		t.Errorf("redirect = %q", redirect)
	}

	pub, err := svc.GetLink(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if !pub.Linked || pub.Login != "coolstreamer" {
		t.Errorf("link = %+v", pub)
	}
}

func TestLinkService_UnlinkBlockedForAcceptingStreamer(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	repo := &fakeLinkRepo{byUser: map[uuid.UUID]domain.TwitchLinkRecord{
		userID: {TwitchLink: domain.TwitchLink{UserID: userID, TwitchUserID: "1", TwitchLogin: "s"}},
	}}

	svc := NewLinkService(
		repo,
		&fakeSettingsRepo{settings: map[uuid.UUID]domain.StreamerSettings{
			userID: {UserID: userID, AcceptingOffers: true},
		}},
		&fakeUserRepo{users: map[uuid.UUID]domain.User{
			userID: {ID: userID, Role: domain.RoleStreamer},
		}},
		nil, "key", "http://ok", true,
	)

	err := svc.Unlink(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
	e, ok := domain.AsError(err)
	if !ok || e.Code != "twitch_unlink_blocked" {
		t.Fatalf("err = %v", err)
	}
}

func TestOAuthClient_AuthorizeURL(t *testing.T) {
	c := NewOAuthClient("id", "secret", "http://cb", nil)
	url := c.AuthorizeURL("state123", "challenge")
	if !strings.Contains(url, "client_id=id") || !strings.Contains(url, "state=state123") {
		t.Errorf("url = %q", url)
	}
}

func TestEncodeDecodeOAuthState(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	state, err := encodeOAuthState("key", userID, "verifier", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	gotID, verifier, err := decodeOAuthState("key", state)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != userID || verifier != "verifier" {
		t.Errorf("got %v %q", gotID, verifier)
	}
}
