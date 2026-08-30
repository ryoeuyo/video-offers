package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleStreamer Role = "streamer"
)

func (r Role) Valid() bool {
	return r == RoleViewer || r == RoleStreamer
}

type User struct {
	ID           uuid.UUID
	Email        string
	Username     string // уникальный, он же слаг публичной предложки
	PasswordHash string
	Role         Role
	DisplayName  string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) IsStreamer() bool { return u.Role == RoleStreamer }

// StreamerSettings существуют только у пользователей с ролью streamer,
// создаются в момент переключения роли.
type StreamerSettings struct {
	UserID              uuid.UUID
	AcceptingOffers     bool
	AllowAnonymous      bool
	MinAccountAge       time.Duration
	RequireTwitchSender bool
	RequireFollow       bool
	MinFollowAge        time.Duration
	RequireSubscription bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (s StreamerSettings) TwitchGatesEnabled() bool {
	return s.RequireTwitchSender || s.RequireFollow || s.RequireSubscription || s.MinFollowAge > 0
}

func (s *StreamerSettings) ApplyGateInvariants() {
	if s.MinFollowAge > 0 {
		s.RequireFollow = true
	}
	if s.RequireFollow || s.RequireSubscription || s.MinFollowAge > 0 {
		s.RequireTwitchSender = true
	}
}

// RefreshToken хранится только хэшем: утечка таблицы не даёт войти под пользователем.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t *RefreshToken) Active(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
