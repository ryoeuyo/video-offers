package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// healthHandler пингует БД: сервис без базы бесполезен, и оркестратор
// должен видеть это как unhealthy, а не как «живой».
func healthHandler(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := contextWithTimeout(c, 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).
				JSON(healthResponse{Status: "degraded", DB: "down"})
		}
		return c.JSON(healthResponse{Status: "ok", DB: "up"})
	}
}
