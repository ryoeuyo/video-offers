package httpapi

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/service"
)

type OfferHandlers struct {
	offers *service.OfferService
}

func NewOfferHandlers(offers *service.OfferService) *OfferHandlers {
	return &OfferHandlers{offers: offers}
}

func (h *OfferHandlers) Create(c *fiber.Ctx) error {
	var req createOfferRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	senderID, err := userID(c)
	if err != nil {
		return err
	}

	offer, err := h.offers.Create(c.UserContext(), c.Params("username"), senderID, service.CreateOfferInput{
		URL:     req.URL,
		Comment: req.Comment,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(toOfferResponse(offer))
}

func (h *OfferHandlers) ListQueue(c *fiber.Ctx) error {
	streamerID, err := userID(c)
	if err != nil {
		return err
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	page, err := h.offers.ListQueue(c.UserContext(), streamerID, c.Query("status"), c.Query("cursor"), limit)
	if err != nil {
		return err
	}
	return c.JSON(toOfferListResponse(page))
}

func (h *OfferHandlers) UpdateStatus(c *fiber.Ctx) error {
	var req updateOfferStatusRequest
	if err := parseBody(c, &req); err != nil {
		return err
	}

	streamerID, err := userID(c)
	if err != nil {
		return err
	}

	offerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrInvalidInput.WithCode("validation_error", "некорректный id")
	}

	offer, err := h.offers.UpdateStatus(c.UserContext(), streamerID, offerID, domain.OfferStatus(req.Status))
	if err != nil {
		return err
	}
	return c.JSON(toOfferResponse(offer))
}

func (h *OfferHandlers) Delete(c *fiber.Ctx) error {
	streamerID, err := userID(c)
	if err != nil {
		return err
	}

	offerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrInvalidInput.WithCode("validation_error", "некорректный id")
	}

	if err := h.offers.DeleteByStreamer(c.UserContext(), streamerID, offerID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *OfferHandlers) ListSent(c *fiber.Ctx) error {
	senderID, err := userID(c)
	if err != nil {
		return err
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	page, err := h.offers.ListSent(c.UserContext(), senderID, c.Query("cursor"), limit)
	if err != nil {
		return err
	}
	return c.JSON(toOfferListResponse(page))
}

func (h *OfferHandlers) RevokeSent(c *fiber.Ctx) error {
	senderID, err := userID(c)
	if err != nil {
		return err
	}

	offerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrInvalidInput.WithCode("validation_error", "некорректный id")
	}

	if err := h.offers.RevokeSent(c.UserContext(), senderID, offerID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func toOfferListResponse(page service.OfferListPage) offerListResponse {
	items := make([]offerResponse, len(page.Items))
	for i, o := range page.Items {
		items[i] = toOfferResponse(o)
	}
	resp := offerListResponse{Items: items}
	if page.NextCursor != "" {
		resp.NextCursor = page.NextCursor
	}
	return resp
}

func toOfferResponse(o domain.Offer) offerResponse {
	resp := offerResponse{
		ID:            o.ID.String(),
		URL:           o.URL,
		NormalizedURL: o.NormalizedURL,
		Provider:      string(o.Provider),
		ExternalID:    o.ExternalID,
		Title:         o.Title,
		ThumbnailURL:  o.ThumbnailURL,
		Comment:       o.Comment,
		Status:        string(o.Status),
		CreatedAt:     o.CreatedAt.UTC().Format(time.RFC3339),
	}
	if o.SenderID != nil {
		s := o.SenderID.String()
		resp.SenderID = &s
	}
	if o.DurationSeconds != nil {
		resp.DurationSeconds = o.DurationSeconds
	}
	if o.WatchedAt != nil {
		s := o.WatchedAt.UTC().Format(time.RFC3339)
		resp.WatchedAt = &s
	}
	return resp
}
