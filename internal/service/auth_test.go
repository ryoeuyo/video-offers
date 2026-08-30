package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/hash"
	jwtpkg "github.com/ruslan/video-offers/internal/pkg/jwt"
	"github.com/ruslan/video-offers/internal/pkg/token"
)

const testJWTSecret = "test-secret-key-at-least-32-bytes-long"

type fakeUserRepo struct {
	byEmail map[string]domain.User
	byID    map[uuid.UUID]domain.User
	createFn func(domain.User) error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byEmail: make(map[string]domain.User),
		byID:    make(map[uuid.UUID]domain.User),
	}
}

func (f *fakeUserRepo) Create(_ context.Context, u domain.User) error {
	if f.createFn != nil {
		return f.createFn(u)
	}
	key := strings.ToLower(u.Email)
	if _, ok := f.byEmail[key]; ok {
		return domain.ErrConflict.WithCode("email_taken", "email уже занят")
	}
	for _, existing := range f.byEmail {
		if strings.EqualFold(existing.Username, u.Username) {
			return domain.ErrConflict.WithCode("username_taken", "username уже занят")
		}
	}
	f.byEmail[key] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := f.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

type fakeRefreshRepo struct {
	byHash map[string]domain.RefreshToken
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{byHash: make(map[string]domain.RefreshToken)}
}

func (f *fakeRefreshRepo) Create(_ context.Context, t domain.RefreshToken) error {
	f.byHash[t.TokenHash] = t
	return nil
}

func (f *fakeRefreshRepo) GetByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	t, ok := f.byHash[hash]
	if !ok {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeRefreshRepo) MarkUsed(_ context.Context, id uuid.UUID, usedAt time.Time) error {
	for hash, t := range f.byHash {
		if t.ID == id {
			if t.UsedAt != nil {
				return domain.ErrUnauthorized.WithCode("invalid_token", "refresh-токен уже использован")
			}
			t.UsedAt = &usedAt
			f.byHash[hash] = t
			return nil
		}
	}
	return domain.ErrUnauthorized.WithCode("invalid_token", "refresh-токен уже использован или не найден")
}

func newTestAuthService(t *testing.T) (*AuthService, *fakeUserRepo, *fakeRefreshRepo, *jwtpkg.Service) {
	t.Helper()
	users := newFakeUserRepo()
	tokens := newFakeRefreshRepo()
	jwtSvc := jwtpkg.New(testJWTSecret, 15*time.Minute)
	svc := NewAuthService(users, tokens, jwtSvc, 720*time.Hour)
	return svc, users, tokens, jwtSvc
}

func TestRegister_Success(t *testing.T) {
	svc, users, tokens, jwtSvc := newTestAuthService(t)
	ctx := context.Background()

	pair, user, err := svc.Register(ctx, RegisterInput{
		Email:    "alice@example.com",
		Username: "alice",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if user.Role != domain.RoleViewer {
		t.Errorf("role = %q, want viewer", user.Role)
	}
	if user.DisplayName != "alice" {
		t.Errorf("display_name = %q, want alice", user.DisplayName)
	}
	if len(users.byEmail) != 1 {
		t.Errorf("users count = %d, want 1", len(users.byEmail))
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if pair.ExpiresIn != int64(jwtSvc.AccessTTL().Seconds()) {
		t.Errorf("expires_in = %d, want %d", pair.ExpiresIn, int64(jwtSvc.AccessTTL().Seconds()))
	}
	if len(tokens.byHash) != 1 {
		t.Errorf("refresh tokens stored = %d, want 1", len(tokens.byHash))
	}

	uid, role, err := jwtSvc.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if uid != user.ID || role != domain.RoleViewer {
		t.Errorf("jwt claims mismatch: id=%v role=%v", uid, role)
	}
}

func TestRegister_ValidationErrors(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	tests := []struct {
		name string
		in   RegisterInput
		code string
	}{
		{
			name: "empty fields",
			in:   RegisterInput{},
			code: "validation_error",
		},
		{
			name: "short password",
			in: RegisterInput{
				Email:    "a@b.c",
				Username: "user1",
				Password: "short",
			},
			code: "validation_error",
		},
		{
			name: "invalid username",
			in: RegisterInput{
				Email:    "a@b.c",
				Username: "bad name",
				Password: "password1",
			},
			code: "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Register(ctx, tt.in)
			assertDomainCode(t, err, tt.code)
			assertErrorKind(t, err, domain.KindInvalidInput)
		})
	}
}

func TestRegister_Conflicts(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, RegisterInput{
		Email:    "dup@example.com",
		Username: "first",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, _, err = svc.Register(ctx, RegisterInput{
		Email:    "dup@example.com",
		Username: "second",
		Password: "password2",
	})
	assertDomainCode(t, err, "email_taken")
	assertErrorKind(t, err, domain.KindConflict)

	_, _, err = svc.Register(ctx, RegisterInput{
		Email:    "other@example.com",
		Username: "first",
		Password: "password2",
	})
	assertDomainCode(t, err, "username_taken")
	assertErrorKind(t, err, domain.KindConflict)
}

func TestLogin_Success(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, registered, err := svc.Register(ctx, RegisterInput{
		Email:    "bob@example.com",
		Username: "bob",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	pair, user, err := svc.Login(ctx, LoginInput{
		Email:    "bob@example.com",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if user.ID != registered.ID {
		t.Errorf("user id mismatch")
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected tokens")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, RegisterInput{
		Email:    "bob@example.com",
		Username: "bob",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	tests := []struct {
		name string
		in   LoginInput
	}{
		{name: "unknown email", in: LoginInput{Email: "ghost@example.com", Password: "password1"}},
		{name: "wrong password", in: LoginInput{Email: "bob@example.com", Password: "wrongpass"}},
		{name: "empty email", in: LoginInput{Email: "", Password: "password1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Login(ctx, tt.in)
			if tt.in.Email == "" {
				assertErrorKind(t, err, domain.KindInvalidInput)
				return
			}
			assertDomainCode(t, err, "invalid_credentials")
			assertErrorKind(t, err, domain.KindUnauthorized)
		})
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	svc, _, tokens, jwtSvc := newTestAuthService(t)
	ctx := context.Background()

	pair, _, err := svc.Register(ctx, RegisterInput{
		Email:    "carol@example.com",
		Username: "carol",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	newPair, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatal("expected new tokens")
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Error("refresh token was not rotated")
	}

	oldHash := token.Hash(pair.RefreshToken)
	stored := tokens.byHash[oldHash]
	if stored.UsedAt == nil {
		t.Error("old refresh token should be marked used")
	}

	uid, _, err := jwtSvc.ParseAccess(newPair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccess new token: %v", err)
	}
	if uid != stored.UserID {
		t.Errorf("user id mismatch in new access token")
	}

	_, err = svc.Refresh(ctx, pair.RefreshToken)
	assertDomainCode(t, err, "invalid_token")
	assertErrorKind(t, err, domain.KindUnauthorized)
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		token string
		kind  domain.Kind
		code  string
	}{
		{name: "empty", token: "", kind: domain.KindInvalidInput, code: "validation_error"},
		{name: "unknown", token: "not-a-real-token", kind: domain.KindUnauthorized, code: "invalid_token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Refresh(ctx, tt.token)
			assertDomainCode(t, err, tt.code)
			assertErrorKind(t, err, tt.kind)
		})
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	svc, users, tokens, _ := newTestAuthService(t)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV7())
	passHash, err := hash.Hash("password1")
	if err != nil {
		t.Fatal(err)
	}
	user := domain.User{
		ID: userID, Email: "exp@example.com", Username: "exp",
		PasswordHash: passHash, Role: domain.RoleViewer,
		DisplayName: "exp", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	users.byEmail[strings.ToLower(user.Email)] = user
	users.byID[user.ID] = user

	plain, hashStr, err := token.Generate()
	if err != nil {
		t.Fatal(err)
	}
	tokenID := uuid.Must(uuid.NewV7())
	tokens.byHash[hashStr] = domain.RefreshToken{
		ID: tokenID, UserID: user.ID, TokenHash: hashStr,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
		CreatedAt: time.Now().UTC(),
	}

	_, err = svc.Refresh(ctx, plain)
	assertDomainCode(t, err, "invalid_token")
	assertErrorKind(t, err, domain.KindUnauthorized)
}

func TestLogout_RevokesToken(t *testing.T) {
	svc, _, tokens, _ := newTestAuthService(t)
	ctx := context.Background()

	pair, _, err := svc.Register(ctx, RegisterInput{
		Email:    "dave@example.com",
		Username: "dave",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := svc.Logout(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	stored := tokens.byHash[token.Hash(pair.RefreshToken)]
	if stored.UsedAt == nil {
		t.Error("token should be revoked")
	}

	_, err = svc.Refresh(ctx, pair.RefreshToken)
	assertDomainCode(t, err, "invalid_token")
}

func TestLogout_Idempotent(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	if err := svc.Logout(ctx, "unknown-token"); err != nil {
		t.Fatalf("Logout unknown: %v", err)
	}

	pair, _, err := svc.Register(ctx, RegisterInput{
		Email:    "eve@example.com",
		Username: "eve",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := svc.Logout(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("first logout: %v", err)
	}
	if err := svc.Logout(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("second logout should be idempotent: %v", err)
	}
}

func TestGetUser(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, user, err := svc.Register(ctx, RegisterInput{
		Email:    "frank@example.com",
		Username: "frank",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := svc.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != user.Email {
		t.Errorf("email = %q, want %q", got.Email, user.Email)
	}

	_, err = svc.GetUser(ctx, uuid.Must(uuid.NewV7()))
	assertErrorKind(t, err, domain.KindNotFound)
}

func assertDomainCode(t *testing.T, err error, want string) {
	t.Helper()
 domErr, ok := domain.AsError(err)
	if !ok {
		t.Fatalf("expected domain error, got %v", err)
	}
	if domErr.Code != want {
		t.Fatalf("code = %q, want %q", domErr.Code, want)
	}
}

func assertErrorKind(t *testing.T, err error, want domain.Kind) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
 domErr, ok := domain.AsError(err)
	if !ok {
		t.Fatalf("expected domain error, got %v", err)
	}
	if domErr.Kind != want {
		t.Fatalf("kind = %v, want %v", domErr.Kind, want)
	}
	if !errors.Is(err, sentinelForKind(want)) {
		t.Fatalf("errors.Is kind mismatch for %v", want)
	}
}

func sentinelForKind(k domain.Kind) error {
	switch k {
	case domain.KindInvalidInput:
		return domain.ErrInvalidInput
	case domain.KindUnauthorized:
		return domain.ErrUnauthorized
	case domain.KindNotFound:
		return domain.ErrNotFound
	case domain.KindConflict:
		return domain.ErrConflict
	case domain.KindForbidden:
		return domain.ErrForbidden
	default:
		return domain.ErrInternal
	}
}
