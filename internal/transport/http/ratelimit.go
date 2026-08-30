package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/domain"
)

func newRateLimiter(max int, window time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return domain.ErrRateLimited
		},
	})
}

func authRateLimit(cfg config.RateLimit) fiber.Handler {
	return newRateLimiter(cfg.AuthMax, cfg.AuthWindow)
}

func offerCreateRateLimit(cfg config.RateLimit) fiber.Handler {
	return newRateLimiter(cfg.OfferCreateMax, cfg.OfferWindow)
}
