package httpapi

import (
	"github.com/gofiber/fiber/v2"

	twitchsvc "github.com/ruslan/video-offers/internal/service/twitch"
)

type TwitchHandlers struct {
	links *twitchsvc.LinkService
}

func NewTwitchHandlers(links *twitchsvc.LinkService) *TwitchHandlers {
	return &TwitchHandlers{links: links}
}

func (h *TwitchHandlers) Connect(c *fiber.Ctx) error {
	userID, err := userID(c)
	if err != nil {
		return err
	}

	url, err := h.links.ConnectURL(c.UserContext(), userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"url": url})
}

func (h *TwitchHandlers) Callback(c *fiber.Ctx) error {
	redirect, err := h.links.HandleCallback(c.UserContext(), c.Query("code"), c.Query("state"))
	if err != nil {
		return err
	}
	return c.Redirect(redirect, fiber.StatusFound)
}

func (h *TwitchHandlers) GetLink(c *fiber.Ctx) error {
	userID, err := userID(c)
	if err != nil {
		return err
	}

	link, err := h.links.GetLink(c.UserContext(), userID)
	if err != nil {
		return err
	}
	return c.JSON(link)
}

func (h *TwitchHandlers) Unlink(c *fiber.Ctx) error {
	userID, err := userID(c)
	if err != nil {
		return err
	}

	if err := h.links.Unlink(c.UserContext(), userID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
