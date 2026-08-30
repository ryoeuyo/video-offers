package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ruslan/video-offers/internal/service"
)

type AuthHandlers struct {
	auth *service.AuthService
}

func NewAuthHandlers(auth *service.AuthService) *AuthHandlers {
	return &AuthHandlers{auth: auth}
}

func (h *AuthHandlers) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	pair, _, err := h.auth.Register(c.UserContext(), service.RegisterInput{
		Email:    req.Email,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(toTokenResponse(pair.AccessToken, pair.RefreshToken, pair.ExpiresIn))
}

func (h *AuthHandlers) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	pair, _, err := h.auth.Login(c.UserContext(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return err
	}

	return c.JSON(toTokenResponse(pair.AccessToken, pair.RefreshToken, pair.ExpiresIn))
}

func (h *AuthHandlers) Refresh(c *fiber.Ctx) error {
	var req refreshRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	pair, err := h.auth.Refresh(c.UserContext(), req.RefreshToken)
	if err != nil {
		return err
	}

	return c.JSON(toTokenResponse(pair.AccessToken, pair.RefreshToken, pair.ExpiresIn))
}

func (h *AuthHandlers) Logout(c *fiber.Ctx) error {
	var req refreshRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	if err := h.auth.Logout(c.UserContext(), req.RefreshToken); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandlers) Me(c *fiber.Ctx) error {
	id, err := userID(c)
	if err != nil {
		return err
	}

	user, err := h.auth.GetUser(c.UserContext(), id)
	if err != nil {
		return err
	}

	return c.JSON(toUserResponse(user))
}
