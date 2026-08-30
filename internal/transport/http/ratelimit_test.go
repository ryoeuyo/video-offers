package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/logger"
)

func TestRateLimit_Returns429(t *testing.T) {
	log := logger.New("dev", "error")
	app := fiber.New(fiber.Config{ErrorHandler: errorHandler(log)})
	app.Post("/", newRateLimiter(1, time.Minute), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	var payload errorBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, domain.ErrRateLimited.Code, payload.Error.Code)
}

func TestAuthRateLimitConfig(t *testing.T) {
	h := authRateLimit(config.RateLimit{AuthMax: 5, AuthWindow: time.Second})
	require.NotNil(t, h)
}
