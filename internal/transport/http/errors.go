package httpapi

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/ruslan/video-offers/internal/domain"
)

// errorBody — единственная форма ошибки, которую видит клиент.
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func statusFor(kind domain.Kind) int {
	switch kind {
	case domain.KindInvalidInput:
		return fiber.StatusBadRequest
	case domain.KindUnauthorized:
		return fiber.StatusUnauthorized
	case domain.KindForbidden:
		return fiber.StatusForbidden
	case domain.KindNotFound:
		return fiber.StatusNotFound
	case domain.KindConflict:
		return fiber.StatusConflict
	case domain.KindUnprocessable:
		return fiber.StatusUnprocessableEntity
	case domain.KindRateLimited:
		return fiber.StatusTooManyRequests
	default:
		return fiber.StatusInternalServerError
	}
}

// errorHandler — единственное место в проекте, где ошибка превращается в HTTP-ответ.
// Handler'ы просто возвращают error.
func errorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if domErr, ok := domain.AsError(err); ok {
			status := statusFor(domErr.Kind)
			if status >= fiber.StatusInternalServerError {
				log.ErrorContext(c.UserContext(), "request failed",
					"error", err, "path", c.Path(), "method", c.Method())
			}
			return c.Status(status).JSON(errorBody{errorPayload{
				Code:    domErr.Code,
				Message: domErr.Message,
				Details: domErr.Details,
			}})
		}

		// Ошибки самого Fiber (404 на неизвестный роут, слишком большое тело и т.п.).
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return c.Status(fiberErr.Code).JSON(errorBody{errorPayload{
				Code:    fiberCode(fiberErr.Code),
				Message: fiberErr.Message,
			}})
		}

		// Всё остальное — наш баг. Наружу ничего, кроме 500, не отдаём.
		log.ErrorContext(c.UserContext(), "unhandled error",
			"error", err, "path", c.Path(), "method", c.Method())
		return c.Status(fiber.StatusInternalServerError).JSON(errorBody{errorPayload{
			Code:    domain.ErrInternal.Code,
			Message: domain.ErrInternal.Message,
		}})
	}
}

func fiberCode(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return "route_not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusRequestEntityTooLarge:
		return "payload_too_large"
	default:
		return "request_error"
	}
}
