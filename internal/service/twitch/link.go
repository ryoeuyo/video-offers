package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/seal"
)

type oauthStatePayload struct {
	UserID       string `json:"user_id"`
	CodeVerifier string `json:"code_verifier"`
	Exp          int64  `json:"exp"`
}

func encodeOAuthState(sealKey string, userID uuid.UUID, codeVerifier string, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(oauthStatePayload{
		UserID:       userID.String(),
		CodeVerifier: codeVerifier,
		Exp:          time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	return seal.EncodeState(sealKey, payload)
}

func decodeOAuthState(sealKey, state string) (uuid.UUID, string, error) {
	raw, err := seal.DecodeState(sealKey, state)
	if err != nil {
		return uuid.UUID{}, "", domain.ErrInvalidInput.WithCode("invalid_oauth_state", "некорректный state").Wrap(err)
	}

	var p oauthStatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return uuid.UUID{}, "", domain.ErrInvalidInput.WithCode("invalid_oauth_state", "некорректный state").Wrap(err)
	}
	if time.Now().Unix() > p.Exp {
		return uuid.UUID{}, "", domain.ErrInvalidInput.WithCode("oauth_state_expired", "state истёк")
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return uuid.UUID{}, "", domain.ErrInvalidInput.WithCode("invalid_oauth_state", "некорректный state")
	}
	if p.CodeVerifier == "" {
		return uuid.UUID{}, "", domain.ErrInvalidInput.WithCode("invalid_oauth_state", "некорректный state")
	}
	return userID, p.CodeVerifier, nil
}

type LinkRepository interface {
	Upsert(ctx context.Context, rec domain.TwitchLinkRecord) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.TwitchLink, error)
	ExistsByUserID(ctx context.Context, userID uuid.UUID) (bool, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}

type StreamerSettingsLookup interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error)
}

type UserRoleLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type LinkService struct {
	repo       LinkRepository
	settings   StreamerSettingsLookup
	users      UserRoleLookup
	oauth      *OAuthClient
	sealKey    string
	stateKey   string
	stateTTL   time.Duration
	successURL string
	enabled    bool
}

func NewLinkService(
	repo LinkRepository,
	settings StreamerSettingsLookup,
	users UserRoleLookup,
	oauth *OAuthClient,
	sealKey string,
	successURL string,
	enabled bool,
) *LinkService {
	return &LinkService{
		repo:       repo,
		settings:   settings,
		users:      users,
		oauth:      oauth,
		sealKey:    sealKey,
		stateKey:   sealKey,
		stateTTL:   10 * time.Minute,
		successURL: successURL,
		enabled:    enabled,
	}
}

func (s *LinkService) Enabled() bool { return s.enabled }

func (s *LinkService) ConnectURL(ctx context.Context, userID uuid.UUID) (string, error) {
	if !s.enabled {
		return "", domain.ErrUnavailable.WithCode("twitch_disabled", "интеграция Twitch не настроена")
	}

	verifier, err := NewCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("code verifier: %w", err)
	}

	state, err := encodeOAuthState(s.stateKey, userID, verifier, s.stateTTL)
	if err != nil {
		return "", fmt.Errorf("oauth state: %w", err)
	}

	return s.oauth.AuthorizeURL(state, CodeChallengeS256(verifier)), nil
}

func (s *LinkService) HandleCallback(ctx context.Context, code, state string) (string, error) {
	if !s.enabled {
		return "", domain.ErrUnavailable.WithCode("twitch_disabled", "интеграция Twitch не настроена")
	}
	if code == "" {
		return "", domain.ErrInvalidInput.WithCode("validation_error", "code обязателен")
	}

	userID, verifier, err := decodeOAuthState(s.stateKey, state)
	if err != nil {
		return "", err
	}

	tokens, err := s.oauth.ExchangeCode(ctx, code, verifier)
	if err != nil {
		return "", domain.ErrUnprocessable.WithCode("twitch_oauth_failed", "не удалось получить токен Twitch").Wrap(err)
	}

	twitchUser, err := s.oauth.GetCurrentUser(ctx, tokens.AccessToken)
	if err != nil {
		return "", domain.ErrUnprocessable.WithCode("twitch_oauth_failed", "не удалось получить профиль Twitch").Wrap(err)
	}

	accessEnc, err := seal.Seal(s.sealKey, tokens.AccessToken)
	if err != nil {
		return "", fmt.Errorf("seal access: %w", err)
	}
	refreshEnc, err := seal.Seal(s.sealKey, tokens.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("seal refresh: %w", err)
	}

	now := time.Now().UTC()
	rec := domain.TwitchLinkRecord{
		TwitchLink: domain.TwitchLink{
			UserID:            userID,
			TwitchUserID:      twitchUser.ID,
			TwitchLogin:       twitchUser.Login,
			TwitchDisplayName: twitchUser.DisplayName,
			Scopes:            tokens.Scope,
			TokenExpiresAt:    now.Add(time.Duration(tokens.ExpiresIn) * time.Second),
			LinkedAt:          now,
			UpdatedAt:         now,
		},
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
	}

	if err := s.repo.Upsert(ctx, rec); err != nil {
		return "", err
	}

	return s.successURL + "?twitch=linked", nil
}

type PublicLink struct {
	Linked      bool   `json:"linked"`
	Login       string `json:"login,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	LinkedAt    string `json:"linked_at,omitempty"`
}

func (s *LinkService) GetLink(ctx context.Context, userID uuid.UUID) (PublicLink, error) {
	link, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if e, ok := domain.AsError(err); ok && e.Kind == domain.KindNotFound {
			return PublicLink{Linked: false}, nil
		}
		return PublicLink{}, err
	}
	return PublicLink{
		Linked:      true,
		Login:       link.TwitchLogin,
		DisplayName: link.TwitchDisplayName,
		LinkedAt:    link.LinkedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *LinkService) HasLink(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.repo.ExistsByUserID(ctx, userID)
}

func (s *LinkService) GetLogin(ctx context.Context, userID uuid.UUID) (string, error) {
	link, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if e, ok := domain.AsError(err); ok && e.Kind == domain.KindNotFound {
			return "", nil
		}
		return "", err
	}
	return link.TwitchLogin, nil
}

func (s *LinkService) Unlink(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Role == domain.RoleStreamer {
		settings, err := s.settings.GetByUserID(ctx, userID)
		if err != nil && !isNotFound(err) {
			return fmt.Errorf("get settings: %w", err)
		}
		if err == nil && settings.AcceptingOffers {
			return domain.ErrConflict.WithCode("twitch_unlink_blocked", "отключите приём офферов перед отвязкой Twitch")
		}
	}

	return s.repo.DeleteByUserID(ctx, userID)
}

func isNotFound(err error) bool {
	e, ok := domain.AsError(err)
	return ok && e.Kind == domain.KindNotFound
}
