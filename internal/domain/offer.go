package domain

import (
	"time"

	"github.com/google/uuid"
)

type Provider string

const (
	ProviderYouTube Provider = "youtube"
	ProviderTwitch  Provider = "twitch"
	ProviderVK      Provider = "vk"
	ProviderOther   Provider = "other"
)

type OfferStatus string

const (
	StatusPending  OfferStatus = "pending"
	StatusWatched  OfferStatus = "watched"
	StatusSkipped  OfferStatus = "skipped"
	StatusRejected OfferStatus = "rejected"
)

func (s OfferStatus) Valid() bool {
	switch s {
	case StatusPending, StatusWatched, StatusSkipped, StatusRejected:
		return true
	}
	return false
}

// CanTransitionTo: из pending можно уйти в любой терминальный статус,
// обратно и между терминальными — нельзя.
func (s OfferStatus) CanTransitionTo(next OfferStatus) bool {
	if s != StatusPending || !next.Valid() {
		return false
	}
	return next != StatusPending
}

type Offer struct {
	ID         uuid.UUID
	StreamerID uuid.UUID
	SenderID   *uuid.UUID // nil — анонимная отправка

	URL           string
	NormalizedURL string // канонический вид, по нему считается дедуп
	Provider      Provider
	ExternalID    string

	Title           string
	ThumbnailURL    string
	DurationSeconds *int
	Comment         string

	Status    OfferStatus
	WatchedAt *time.Time
	CreatedAt time.Time
}

// VideoMeta — то, что резолвер достаёт у провайдера. Пустой meta допустим:
// оффер создаётся и с голой ссылкой.
type VideoMeta struct {
	Provider        Provider
	ExternalID      string
	Title           string
	ThumbnailURL    string
	DurationSeconds *int
}
