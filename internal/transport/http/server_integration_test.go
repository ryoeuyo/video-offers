package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/jwt"
	"github.com/ruslan/video-offers/internal/pkg/logger"
	"github.com/ruslan/video-offers/internal/repo"
	"github.com/ruslan/video-offers/internal/service"
	"github.com/ruslan/video-offers/internal/service/video"
	"github.com/ruslan/video-offers/internal/testutil"
	httpapi "github.com/ruslan/video-offers/internal/transport/http"
)

type stubVideoResolver struct{}

func (stubVideoResolver) Resolve(_ context.Context, p video.ParsedURL) (domain.VideoMeta, error) {
	return domain.VideoMeta{
		Provider:     p.Provider,
		ExternalID:   p.ExternalID,
		Title:        "Integration Test Video",
		ThumbnailURL: "https://example.com/thumb.jpg",
	}, nil
}

func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()

	pool, cleanup := testutil.PostgresContainer(t)
	t.Cleanup(cleanup)

	cfg := config.Config{
		Env:      "dev",
		LogLevel: "error",
		Auth: config.Auth{
			JWTSecret:  "integration-test-secret-at-least-32-bytes",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 720 * time.Hour,
		},
		RateLimit: config.RateLimit{
			AuthMax:        1000,
			AuthWindow:     time.Minute,
			OfferCreateMax: 1000,
			OfferWindow:    time.Minute,
		},
	}

	log := logger.New(cfg.Env, cfg.LogLevel)
	userRepo := repo.NewUserRepo(pool)
	refreshRepo := repo.NewRefreshTokenRepo(pool)
	settingsRepo := repo.NewStreamerSettingsRepo(pool)
	jwtSvc := jwt.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL)
	authSvc := service.NewAuthService(userRepo, refreshRepo, jwtSvc, cfg.Auth.RefreshTTL)
	userSvc := service.NewUserService(userRepo, settingsRepo, nil)
	offerRepo := repo.NewOfferRepo(pool)
	offerSvc := service.NewOfferService(offerRepo, userRepo, settingsRepo, stubVideoResolver{}, nil)

	return httpapi.NewServer(cfg, log, httpapi.Deps{
		Pool:   pool,
		Auth:   authSvc,
		Users:  userSvc,
		Offers: offerSvc,
		JWT:    jwtSvc,
	})
}

func jsonRequest(t *testing.T, method, target, token string, body any) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func decodeJSON(t *testing.T, body io.Reader, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(body).Decode(dst))
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type errorResp struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

type offerResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type offerListResp struct {
	Items []offerResp `json:"items"`
}

func TestIntegration_MVPFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	srv := newTestServer(t)
	app := srv.App()

	// healthz
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// register streamer
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "streamer@example.com", "username": "alice", "password": "password1",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	var streamerTokens tokenResp
	decodeJSON(t, resp.Body, &streamerTokens)

	// become streamer
	resp, err = app.Test(jsonRequest(t, http.MethodPatch, "/api/v1/me", streamerTokens.AccessToken, map[string]string{
		"role": "streamer",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// register viewer
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "viewer@example.com", "username": "bob", "password": "password1",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	var viewerTokens tokenResp
	decodeJSON(t, resp.Body, &viewerTokens)

	// public profile
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/streamers/alice", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// create offer
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/streamers/alice/offers", viewerTokens.AccessToken, map[string]string{
		"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "comment": "nice",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	var created offerResp
	decodeJSON(t, resp.Body, &created)

	// duplicate
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/streamers/alice/offers", viewerTokens.AccessToken, map[string]string{
		"url": "https://youtu.be/dQw4w9WgXcQ",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusConflict, resp.StatusCode)
	var dupErr errorResp
	decodeJSON(t, resp.Body, &dupErr)
	require.Equal(t, "offer_duplicate", dupErr.Error.Code)

	// streamer queue
	req := jsonRequest(t, http.MethodGet, "/api/v1/me/offers?status=pending", streamerTokens.AccessToken, nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	var queue offerListResp
	decodeJSON(t, resp.Body, &queue)
	require.Len(t, queue.Items, 1)

	// mark watched
	resp, err = app.Test(jsonRequest(t, http.MethodPatch, "/api/v1/me/offers/"+created.ID, streamerTokens.AccessToken, map[string]string{
		"status": "watched",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// viewer sent list
	resp, err = app.Test(jsonRequest(t, http.MethodGet, "/api/v1/me/sent", viewerTokens.AccessToken, nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	var sent offerListResp
	decodeJSON(t, resp.Body, &sent)
	require.Len(t, sent.Items, 1)
	require.Equal(t, "watched", sent.Items[0].Status)

	// refresh
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": viewerTokens.RefreshToken,
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	var refreshed tokenResp
	decodeJSON(t, resp.Body, &refreshed)

	// logout
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/logout", refreshed.AccessToken, map[string]string{
		"refresh_token": refreshed.RefreshToken,
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	// refresh after logout fails
	resp, err = app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refreshed.RefreshToken,
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_EndpointsSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	srv := newTestServer(t)
	app := srv.App()

	resp, err := app.Test(jsonRequest(t, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "smoke@example.com", "username": "smokeuser", "password": "password1",
	}))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	var tokens tokenResp
	decodeJSON(t, resp.Body, &tokens)

	tests := []struct {
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{http.MethodGet, "/api/v1/me", tokens.AccessToken, nil, fiber.StatusOK},
		{http.MethodPatch, "/api/v1/me", tokens.AccessToken, map[string]string{"display_name": "Smoke"}, fiber.StatusOK},
		{http.MethodGet, "/api/v1/streamers", "", nil, fiber.StatusOK},
		{http.MethodGet, "/api/v1/me/sent", tokens.AccessToken, nil, fiber.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, err := app.Test(jsonRequest(t, tt.method, tt.path, tt.token, tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, resp.StatusCode)
		})
	}
}
