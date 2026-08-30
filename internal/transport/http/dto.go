package httpapi

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/service"
)

type registerRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type updateMeRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,min=1,max=100"`
	AvatarURL   *string `json:"avatar_url" validate:"omitempty,max=2048"`
	Role        *string `json:"role" validate:"omitempty,oneof=viewer streamer"`
}

type settingsResponse struct {
	AcceptingOffers      bool `json:"accepting_offers"`
	AllowAnonymous       bool `json:"allow_anonymous"`
	MinAccountAgeSeconds int  `json:"min_account_age_seconds"`
}

type updateSettingsRequest struct {
	AcceptingOffers *bool `json:"accepting_offers"`
}

type streamerPublicResponse struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	AvatarURL       string `json:"avatar_url"`
	AcceptingOffers bool   `json:"accepting_offers"`
}

type listResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type createOfferRequest struct {
	URL     string `json:"url" validate:"required,max=2048"`
	Comment string `json:"comment" validate:"max=500"`
}

type offerResponse struct {
	ID              string  `json:"id"`
	SenderID        *string `json:"sender_id,omitempty"`
	URL             string  `json:"url"`
	NormalizedURL   string  `json:"normalized_url"`
	Provider        string  `json:"provider"`
	ExternalID      string  `json:"external_id"`
	Title           string  `json:"title"`
	ThumbnailURL    string  `json:"thumbnail_url"`
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	Comment         string  `json:"comment"`
	Status          string  `json:"status"`
	WatchedAt       *string `json:"watched_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type updateOfferStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=watched skipped rejected"`
}

type offerListResponse struct {
	Items      []offerResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

func parseBody(c *fiber.Ctx, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return domain.ErrInvalidInput.WithCode("invalid_json", "некорректный JSON").Wrap(err)
	}
	if err := getValidator().Struct(dst); err != nil {
		return domain.ErrInvalidInput.WithCode("validation_error", "ошибка валидации").
			WithDetails(validationDetails(err))
	}
	return nil
}

func toTokenResponse(access, refresh string, expiresIn int64) tokenResponse {
	return tokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    expiresIn,
	}
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		Username:    u.Username,
		Role:        string(u.Role),
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSettingsResponse(s domain.StreamerSettings) settingsResponse {
	return settingsResponse{
		AcceptingOffers:      s.AcceptingOffers,
		AllowAnonymous:       s.AllowAnonymous,
		MinAccountAgeSeconds: int(s.MinAccountAge / time.Second),
	}
}

func toStreamerPublicResponse(s service.PublicStreamer) streamerPublicResponse {
	return streamerPublicResponse{
		Username:        s.Username,
		DisplayName:     s.DisplayName,
		AvatarURL:       s.AvatarURL,
		AcceptingOffers: s.AcceptingOffers,
	}
}

func validationDetails(err error) map[string]any {
	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fe.Field()] = fe.Tag()
	}
	return map[string]any{"fields": fields}
}
