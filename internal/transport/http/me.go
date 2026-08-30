package httpapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/service"
)

type MeHandlers struct {
	users *service.UserService
}

func NewMeHandlers(users *service.UserService) *MeHandlers {
	return &MeHandlers{users: users}
}

func (h *MeHandlers) Update(c *fiber.Ctx) error {
	var req updateMeRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	userID, err := userID(c)
	if err != nil {
		return err
	}

	in := service.UpdateProfileInput{
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	}
	if req.Role != nil {
		role := domain.Role(*req.Role)
		in.Role = &role
	}

	user, err := h.users.UpdateProfile(c.UserContext(), userID, in)
	if err != nil {
		return err
	}
	return c.JSON(toUserResponse(user))
}

func (h *MeHandlers) GetSettings(c *fiber.Ctx) error {
	userID, err := userID(c)
	if err != nil {
		return err
	}

	settings, err := h.users.GetSettings(c.UserContext(), userID)
	if err != nil {
		return err
	}
	return c.JSON(toSettingsResponse(settings))
}

func (h *MeHandlers) UpdateSettings(c *fiber.Ctx) error {
	var req updateSettingsRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	userID, err := userID(c)
	if err != nil {
		return err
	}

	settings, err := h.users.UpdateSettings(c.UserContext(), userID, service.UpdateSettingsInput{
		AcceptingOffers:      req.AcceptingOffers,
		MinAccountAgeSeconds: req.MinAccountAgeSeconds,
		RequireTwitchSender:  req.RequireTwitchSender,
		RequireFollow:        req.RequireFollow,
		MinFollowAgeSeconds:  req.MinFollowAgeSeconds,
		RequireSubscription:  req.RequireSubscription,
	})
	if err != nil {
		return err
	}
	return c.JSON(toSettingsResponse(settings))
}

type StreamerHandlers struct {
	users *service.UserService
}

func NewStreamerHandlers(users *service.UserService) *StreamerHandlers {
	return &StreamerHandlers{users: users}
}

func (h *StreamerHandlers) List(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	q := c.Query("q")
	cursor := c.Query("cursor")

	page, err := h.users.ListStreamers(c.UserContext(), q, cursor, limit)
	if err != nil {
		return err
	}

	items := make([]streamerPublicResponse, len(page.Items))
	for i, s := range page.Items {
		items[i] = toStreamerPublicResponse(s)
	}

	resp := listResponse[streamerPublicResponse]{
		Items: items,
	}
	if page.NextCursor != "" {
		resp.NextCursor = page.NextCursor
	}
	return c.JSON(resp)
}

func (h *StreamerHandlers) GetByUsername(c *fiber.Ctx) error {
	username := c.Params("username")
	profile, err := h.users.GetPublicStreamer(c.UserContext(), username)
	if err != nil {
		return err
	}
	return c.JSON(toStreamerPublicResponse(profile))
}
