package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/ruslan/video-offers/internal/domain"
	jwtpkg "github.com/ruslan/video-offers/internal/pkg/jwt"
)

// RequireAuth парсит Bearer access-токен и кладёт userID + role в locals.
func RequireAuth(jwt *jwtpkg.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return domain.ErrUnauthorized
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			return domain.ErrUnauthorized.WithCode("invalid_token", "ожидается Authorization: Bearer")
		}

		tokenStr := strings.TrimSpace(header[len(prefix):])
		if tokenStr == "" {
			return domain.ErrUnauthorized
		}

		userID, role, err := jwt.ParseAccess(tokenStr)
		if err != nil {
			return err
		}

		setUser(c, userID, role)
		return c.Next()
	}
}

// RequireStreamer проверяет, что пользователь — streamer.
func RequireStreamer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if userRole(c) != domain.RoleStreamer {
			return domain.ErrForbidden.WithCode("streamer_required", "требуется роль streamer")
		}
		return c.Next()
	}
}
