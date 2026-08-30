package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/hash"
	jwtpkg "github.com/ruslan/video-offers/internal/pkg/jwt"
	"github.com/ruslan/video-offers/internal/pkg/token"
	pkguuid "github.com/ruslan/video-offers/internal/pkg/uuid"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t domain.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // секунды до протухания access
}

type RegisterInput struct {
	Email    string
	Username string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthService struct {
	users    UserRepository
	tokens   RefreshTokenRepository
	jwt      *jwtpkg.Service
	refreshTTL time.Duration
}

func NewAuthService(
	users UserRepository,
	tokens RefreshTokenRepository,
	jwt *jwtpkg.Service,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		jwt:        jwt,
		refreshTTL: refreshTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (TokenPair, domain.User, error) {
	if err := validateRegister(in); err != nil {
		return TokenPair{}, domain.User{}, err
	}

	passHash, err := hash.Hash(in.Password)
	if err != nil {
		return TokenPair{}, domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	id, err := pkguuid.New()
	if err != nil {
		return TokenPair{}, domain.User{}, err
	}

	user := domain.User{
		ID:           id,
		Email:        strings.TrimSpace(in.Email),
		Username:     strings.TrimSpace(in.Username),
		PasswordHash: passHash,
		Role:         domain.RoleViewer,
		DisplayName:  strings.TrimSpace(in.Username),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return TokenPair{}, domain.User{}, err
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return TokenPair{}, domain.User{}, err
	}
	return pair, user, nil
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (TokenPair, domain.User, error) {
	email := strings.TrimSpace(in.Email)
	if email == "" || in.Password == "" {
		return TokenPair{}, domain.User{}, domain.ErrInvalidInput.WithCode("validation_error", "email и password обязательны")
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return TokenPair{}, domain.User{}, domain.ErrUnauthorized.WithCode("invalid_credentials", "неверный email или пароль")
		}
		return TokenPair{}, domain.User{}, fmt.Errorf("get user: %w", err)
	}

	if err := hash.Verify(in.Password, user.PasswordHash); err != nil {
		if errors.Is(err, hash.ErrMismatch) {
			return TokenPair{}, domain.User{}, domain.ErrUnauthorized.WithCode("invalid_credentials", "неверный email или пароль")
		}
		return TokenPair{}, domain.User{}, fmt.Errorf("verify password: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return TokenPair{}, domain.User{}, err
	}
	return pair, user, nil
}

// Refresh ротирует refresh-токен: старый помечается used, выдаётся новая пара.
func (s *AuthService) Refresh(ctx context.Context, refreshPlain string) (TokenPair, error) {
	if refreshPlain == "" {
		return TokenPair{}, domain.ErrInvalidInput.WithCode("validation_error", "refresh_token обязателен")
	}

	now := time.Now().UTC()
	hashStr := token.Hash(refreshPlain)

	stored, err := s.tokens.GetByHash(ctx, hashStr)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return TokenPair{}, domain.ErrUnauthorized.WithCode("invalid_token", "недействительный refresh-токен")
		}
		return TokenPair{}, fmt.Errorf("get refresh token: %w", err)
	}

	if !stored.Active(now) {
		return TokenPair{}, domain.ErrUnauthorized.WithCode("invalid_token", "refresh-токен истёк или уже использован")
	}

	if err := s.tokens.MarkUsed(ctx, stored.ID, now); err != nil {
		return TokenPair{}, err
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("get user: %w", err)
	}

	return s.issueTokenPair(ctx, user)
}

// Logout отзывает refresh-тoken. Идемпотентен: повторный вызов не ошибка.
func (s *AuthService) Logout(ctx context.Context, refreshPlain string) error {
	if refreshPlain == "" {
		return domain.ErrInvalidInput.WithCode("validation_error", "refresh_token обязателен")
	}

	now := time.Now().UTC()
	hashStr := token.Hash(refreshPlain)

	stored, err := s.tokens.GetByHash(ctx, hashStr)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get refresh token: %w", err)
	}

	if !stored.Active(now) {
		return nil
	}

	if err := s.tokens.MarkUsed(ctx, stored.ID, now); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil
		}
		return err
	}
	return nil
}

func (s *AuthService) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, user domain.User) (TokenPair, error) {
	access, _, err := s.jwt.IssueAccess(user.ID, user.Role)
	if err != nil {
		return TokenPair{}, err
	}

	plain, hashStr, err := token.Generate()
	if err != nil {
		return TokenPair{}, err
	}

	tokenID, err := pkguuid.New()
	if err != nil {
		return TokenPair{}, err
	}

	now := time.Now().UTC()
	rt := domain.RefreshToken{
		ID:        tokenID,
		UserID:    user.ID,
		TokenHash: hashStr,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return TokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: plain,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
	}, nil
}

func validateRegister(in RegisterInput) error {
	email := strings.TrimSpace(in.Email)
	username := strings.TrimSpace(in.Username)

	if email == "" || username == "" || in.Password == "" {
		return domain.ErrInvalidInput.WithCode("validation_error", "email, username и password обязательны")
	}
	if len(in.Password) < 8 {
		return domain.ErrInvalidInput.WithCode("validation_error", "password минимум 8 символов").
			WithDetails(map[string]any{"field": "password"})
	}
	if !usernameRe.MatchString(username) {
		return domain.ErrInvalidInput.WithCode("validation_error", "username: 3–32 символа, латиница, цифры, _").
			WithDetails(map[string]any{"field": "username"})
	}
	return nil
}
