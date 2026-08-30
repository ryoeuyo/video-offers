package domain

import (
	"time"

	"github.com/google/uuid"
)

// TwitchLink — привязка OfferBox-пользователя к Twitch (без секретов).
type TwitchLink struct {
	UserID            uuid.UUID
	TwitchUserID      string
	TwitchLogin       string
	TwitchDisplayName string
	Scopes            []string
	TokenExpiresAt    time.Time
	LinkedAt          time.Time
	UpdatedAt         time.Time
}

// TwitchLinkRecord — полная запись с зашифрованными токенами для repo.
type TwitchLinkRecord struct {
	TwitchLink
	AccessTokenEnc  []byte
	RefreshTokenEnc []byte
}
