package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
)

type Claims struct {
	Role domain.Role `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	secret    []byte
	accessTTL time.Duration
}

func New(secret string, accessTTL time.Duration) *Service {
	return &Service{secret: []byte(secret), accessTTL: accessTTL}
}

func (s *Service) AccessTTL() time.Duration { return s.accessTTL }

// IssueAccess создаёт JWT access-токен с sub=userID и role.
func (s *Service) IssueAccess(userID uuid.UUID, role domain.Role) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(s.accessTTL)

	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// ParseAccess валидирует access-токен и возвращает userID и role.
func (s *Service) ParseAccess(tokenStr string) (uuid.UUID, domain.Role, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return uuid.Nil, "", domain.ErrUnauthorized.WithCode("invalid_token", "недействительный токен").Wrap(err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return uuid.Nil, "", domain.ErrUnauthorized.WithCode("invalid_token", "недействительный токен")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", domain.ErrUnauthorized.WithCode("invalid_token", "недействительный токен").Wrap(err)
	}
	if !claims.Role.Valid() {
		return uuid.Nil, "", domain.ErrUnauthorized.WithCode("invalid_token", "недействительный токен")
	}

	return userID, claims.Role, nil
}
