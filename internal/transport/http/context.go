package httpapi

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
)

// Ключи в c.Locals. Свой тип, чтобы не пересечься с ключами middleware.
type localKey string

const (
	localUserID localKey = "user_id"
	localRole   localKey = "role"
)

func contextWithTimeout(c *fiber.Ctx, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), d)
}

func setUser(c *fiber.Ctx, id uuid.UUID, role domain.Role) {
	c.Locals(localUserID, id)
	c.Locals(localRole, role)
}

// userID достаёт пользователя, положенного requireAuth. Отсутствие — не 401,
// а баг в роутинге: значит, хендлер повесили без middleware.
func userID(c *fiber.Ctx) (uuid.UUID, error) {
	id, ok := c.Locals(localUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return id, nil
}

func userRole(c *fiber.Ctx) domain.Role {
	role, _ := c.Locals(localRole).(domain.Role)
	return role
}
