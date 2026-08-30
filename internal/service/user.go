package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/pagination"
	"github.com/ruslan/video-offers/internal/repo"
)

type ProfileUserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
	Update(ctx context.Context, u domain.User) error
	ListStreamers(ctx context.Context, p repo.ListStreamersParams) ([]repo.StreamerListItem, error)
}

type SettingsRepository interface {
	Create(ctx context.Context, s domain.StreamerSettings) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error)
	Update(ctx context.Context, s domain.StreamerSettings) error
}

type UpdateProfileInput struct {
	DisplayName *string
	AvatarURL   *string
	Role        *domain.Role
}

type UpdateSettingsInput struct {
	AcceptingOffers      *bool
	MinAccountAgeSeconds *int
	RequireTwitchSender  *bool
	RequireFollow        *bool
	MinFollowAgeSeconds  *int
	RequireSubscription  *bool
}

func (in UpdateSettingsInput) empty() bool {
	return in.AcceptingOffers == nil &&
		in.MinAccountAgeSeconds == nil &&
		in.RequireTwitchSender == nil &&
		in.RequireFollow == nil &&
		in.MinFollowAgeSeconds == nil &&
		in.RequireSubscription == nil
}

type TwitchLinkChecker interface {
	HasLink(ctx context.Context, userID uuid.UUID) (bool, error)
	GetLogin(ctx context.Context, userID uuid.UUID) (string, error)
	HasScope(ctx context.Context, userID uuid.UUID, scope string) (bool, error)
}

type OfferRules struct {
	MinAccountAgeSeconds int
	RequireTwitchSender  bool
	RequireFollow        bool
	MinFollowAgeSeconds  int
	RequireSubscription  bool
}

type PublicStreamer struct {
	Username        string
	DisplayName     string
	AvatarURL       string
	AcceptingOffers bool
	TwitchLogin     string
	OfferRules      OfferRules
}

type StreamerListPage struct {
	Items      []PublicStreamer
	NextCursor string
}

type UserService struct {
	users    ProfileUserRepository
	settings SettingsRepository
	twitch   TwitchLinkChecker
}

func NewUserService(users ProfileUserRepository, settings SettingsRepository, twitch TwitchLinkChecker) *UserService {
	return &UserService{users: users, settings: settings, twitch: normalizeTwitchChecker(twitch)}
}

// normalizeTwitchChecker убирает typed-nil (*T)(nil) из интерфейса — иначе twitch != nil, но вызов паникует.
func normalizeTwitchChecker(t TwitchLinkChecker) TwitchLinkChecker {
	if t == nil {
		return nil
	}
	v := reflect.ValueOf(t)
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return t
}

func (s *UserService) twitchConfigured() bool {
	return s.twitch != nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (domain.User, error) {
	if in.DisplayName == nil && in.AvatarURL == nil && in.Role == nil {
		return domain.User{}, domain.ErrInvalidInput.WithCode("validation_error", "нет полей для обновления")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if name == "" {
			return domain.User{}, domain.ErrInvalidInput.WithCode("validation_error", "display_name не может быть пустым").
				WithDetails(map[string]any{"field": "display_name"})
		}
		if len(name) > 100 {
			return domain.User{}, domain.ErrInvalidInput.WithCode("validation_error", "display_name максимум 100 символов").
				WithDetails(map[string]any{"field": "display_name"})
		}
		user.DisplayName = name
	}

	if in.AvatarURL != nil {
		avatar := strings.TrimSpace(*in.AvatarURL)
		if avatar != "" {
			if err := validateAvatarURL(avatar); err != nil {
				return domain.User{}, err
			}
		}
		user.AvatarURL = avatar
	}

	if in.Role != nil {
		if !in.Role.Valid() {
			return domain.User{}, domain.ErrInvalidInput.WithCode("validation_error", "role должен быть viewer или streamer").
				WithDetails(map[string]any{"field": "role"})
		}
		if user.Role != *in.Role {
			if err := s.applyRoleChange(ctx, &user, *in.Role); err != nil {
				return domain.User{}, err
			}
		}
	}

	user.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (s *UserService) applyRoleChange(ctx context.Context, user *domain.User, role domain.Role) error {
	if role == domain.RoleStreamer && user.Role != domain.RoleStreamer {
		_, err := s.settings.GetByUserID(ctx, user.ID)
		if errors.Is(err, domain.ErrNotFound) {
			now := time.Now().UTC()
			accepting := true
			if s.twitchConfigured() {
				linked, linkErr := s.twitch.HasLink(ctx, user.ID)
				if linkErr != nil {
					return fmt.Errorf("check twitch link: %w", linkErr)
				}
				accepting = linked
			}
			if err := s.settings.Create(ctx, domain.StreamerSettings{
				UserID:          user.ID,
				AcceptingOffers: accepting,
				AllowAnonymous:  false,
				MinAccountAge:   0,
				CreatedAt:       now,
				UpdatedAt:       now,
			}); err != nil {
				return fmt.Errorf("create streamer settings: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("get streamer settings: %w", err)
		}
	}
	user.Role = role
	return nil
}

func (s *UserService) GetSettings(ctx context.Context, userID uuid.UUID) (domain.StreamerSettings, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.StreamerSettings{}, err
	}
	if user.Role != domain.RoleStreamer {
		return domain.StreamerSettings{}, domain.ErrForbidden.WithCode("streamer_required", "требуется роль streamer")
	}
	return s.settings.GetByUserID(ctx, userID)
}

func (s *UserService) UpdateSettings(ctx context.Context, userID uuid.UUID, in UpdateSettingsInput) (domain.StreamerSettings, error) {
	if in.empty() {
		return domain.StreamerSettings{}, domain.ErrInvalidInput.WithCode("validation_error", "нет полей для обновления")
	}
	if err := validateNonNegativeSeconds(in.MinAccountAgeSeconds, "min_account_age_seconds"); err != nil {
		return domain.StreamerSettings{}, err
	}
	if err := validateNonNegativeSeconds(in.MinFollowAgeSeconds, "min_follow_age_seconds"); err != nil {
		return domain.StreamerSettings{}, err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.StreamerSettings{}, err
	}
	if user.Role != domain.RoleStreamer {
		return domain.StreamerSettings{}, domain.ErrForbidden.WithCode("streamer_required", "требуется роль streamer")
	}

	settings, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return domain.StreamerSettings{}, err
	}

	if in.AcceptingOffers != nil {
		settings.AcceptingOffers = *in.AcceptingOffers
	}
	if in.MinAccountAgeSeconds != nil {
		settings.MinAccountAge = time.Duration(*in.MinAccountAgeSeconds) * time.Second
	}
	if in.RequireTwitchSender != nil {
		settings.RequireTwitchSender = *in.RequireTwitchSender
	}
	if in.RequireFollow != nil {
		settings.RequireFollow = *in.RequireFollow
	}
	if in.MinFollowAgeSeconds != nil {
		settings.MinFollowAge = time.Duration(*in.MinFollowAgeSeconds) * time.Second
	}
	if in.RequireSubscription != nil {
		settings.RequireSubscription = *in.RequireSubscription
	}
	settings.ApplyGateInvariants()
	settings.UpdatedAt = time.Now().UTC()

	if settings.AcceptingOffers && s.twitchConfigured() {
		if err := s.requireStreamerTwitch(ctx, userID); err != nil {
			return domain.StreamerSettings{}, err
		}
	}
	if settings.TwitchGatesEnabled() {
		if err := s.requireStreamerTwitch(ctx, userID); err != nil {
			return domain.StreamerSettings{}, err
		}
	}
	if settings.RequireSubscription {
		if err := s.requireSubscriptionScope(ctx, userID); err != nil {
			return domain.StreamerSettings{}, err
		}
	}

	if err := s.settings.Update(ctx, settings); err != nil {
		return domain.StreamerSettings{}, fmt.Errorf("update settings: %w", err)
	}
	return settings, nil
}

func validateNonNegativeSeconds(v *int, field string) error {
	if v != nil && *v < 0 {
		return domain.ErrInvalidInput.WithCode("validation_error", field+" не может быть отрицательным").
			WithDetails(map[string]any{"field": field})
	}
	return nil
}

func (s *UserService) requireStreamerTwitch(ctx context.Context, userID uuid.UUID) error {
	if !s.twitchConfigured() {
		return domain.ErrConflict.WithCode("twitch_required", "привяжите Twitch, чтобы принимать офферы")
	}
	linked, err := s.twitch.HasLink(ctx, userID)
	if err != nil {
		return fmt.Errorf("check twitch link: %w", err)
	}
	if !linked {
		return domain.ErrConflict.WithCode("twitch_required", "привяжите Twitch, чтобы принимать офферы")
	}
	return nil
}

func (s *UserService) requireSubscriptionScope(ctx context.Context, userID uuid.UUID) error {
	if !s.twitchConfigured() {
		return domain.ErrConflict.WithCode("twitch_required", "привяжите Twitch, чтобы принимать офферы")
	}
	ok, err := s.twitch.HasScope(ctx, userID, domain.TwitchScopeChannelSubscriptions)
	if err != nil {
		return fmt.Errorf("check twitch scope: %w", err)
	}
	if !ok {
		return domain.ErrConflict.WithCode("twitch_scope_required", "перепривяжите Twitch с доступом к подпискам канала")
	}
	return nil
}

func (s *UserService) GetPublicStreamer(ctx context.Context, username string) (PublicStreamer, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return PublicStreamer{}, domain.ErrInvalidInput.WithCode("validation_error", "username обязателен")
	}

	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return PublicStreamer{}, err
	}
	if user.Role != domain.RoleStreamer {
		return PublicStreamer{}, domain.ErrNotFound.WithCode("streamer_not_found", "стример не найден")
	}

	settings, err := s.settings.GetByUserID(ctx, user.ID)
	if err != nil {
		return PublicStreamer{}, fmt.Errorf("get streamer settings: %w", err)
	}

	return toPublicStreamer(user, settings, s.twitchLogin(ctx, user.ID)), nil
}

func (s *UserService) twitchLogin(ctx context.Context, userID uuid.UUID) string {
	if !s.twitchConfigured() {
		return ""
	}
	login, err := s.twitch.GetLogin(ctx, userID)
	if err != nil {
		return ""
	}
	return login
}

func (s *UserService) ListStreamers(ctx context.Context, usernamePrefix, cursorRaw string, limit int) (StreamerListPage, error) {
	limit = pagination.ClampLimit(limit, 20, 100)
	prefix := strings.TrimSpace(usernamePrefix)

	var cursor *pagination.Key
	if cursorRaw != "" {
		key, err := pagination.Decode(cursorRaw)
		if err != nil {
			return StreamerListPage{}, domain.ErrInvalidInput.WithCode("invalid_cursor", "некорректный cursor").Wrap(err)
		}
		cursor = &key
	}

	items, err := s.users.ListStreamers(ctx, repo.ListStreamersParams{
		UsernamePrefix: prefix,
		Cursor:         cursor,
		Limit:          limit,
	})
	if err != nil {
		return StreamerListPage{}, err
	}

	page := StreamerListPage{Items: make([]PublicStreamer, 0, len(items))}
	for i, item := range items {
		if i >= limit {
			last := items[limit-1]
			page.NextCursor = pagination.Encode(pagination.Key{
				CreatedAt: last.User.CreatedAt,
				ID:        last.User.ID,
			})
			break
		}
		page.Items = append(page.Items, toPublicStreamer(item.User, domain.StreamerSettings{AcceptingOffers: item.AcceptingOffers}, s.twitchLogin(ctx, item.User.ID)))
	}

	return page, nil
}

func toPublicStreamer(u domain.User, settings domain.StreamerSettings, twitchLogin string) PublicStreamer {
	return PublicStreamer{
		Username:        u.Username,
		DisplayName:     u.DisplayName,
		AvatarURL:       u.AvatarURL,
		AcceptingOffers: settings.AcceptingOffers,
		TwitchLogin:     twitchLogin,
		OfferRules: OfferRules{
			MinAccountAgeSeconds: int(settings.MinAccountAge / time.Second),
			RequireTwitchSender:  settings.RequireTwitchSender,
			RequireFollow:        settings.RequireFollow,
			MinFollowAgeSeconds:  int(settings.MinFollowAge / time.Second),
			RequireSubscription:  settings.RequireSubscription,
		},
	}
}

func validateAvatarURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return domain.ErrInvalidInput.WithCode("validation_error", "avatar_url должен быть валидным URL").
			WithDetails(map[string]any{"field": "avatar_url"})
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return domain.ErrInvalidInput.WithCode("validation_error", "avatar_url должен быть http или https").
			WithDetails(map[string]any{"field": "avatar_url"})
	}
	if len(raw) > 2048 {
		return domain.ErrInvalidInput.WithCode("validation_error", "avatar_url слишком длинный").
			WithDetails(map[string]any{"field": "avatar_url"})
	}
	return nil
}
