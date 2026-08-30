package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ruslan/video-offers/internal/domain"
)

// Resolver достаёт метаданные видео у провайдера.
type Resolver interface {
	Resolve(ctx context.Context, p ParsedURL) (domain.VideoMeta, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type YouTubeResolver struct {
	client  HTTPClient
	baseURL string // для тестов
}

func NewYouTubeResolver(client HTTPClient) *YouTubeResolver {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &YouTubeResolver{client: client, baseURL: "https://www.youtube.com/oembed"}
}

type oembedResponse struct {
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func (r *YouTubeResolver) Resolve(ctx context.Context, p ParsedURL) (domain.VideoMeta, error) {
	meta := domain.VideoMeta{
		Provider:   domain.ProviderYouTube,
		ExternalID: p.ExternalID,
	}

	endpoint, err := url.Parse(r.baseURL)
	if err != nil {
		return meta, fmt.Errorf("parse oembed url: %w", err)
	}
	q := endpoint.Query()
	q.Set("url", p.Normalized)
	q.Set("format", "json")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return meta, fmt.Errorf("build oembed request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return meta, fmt.Errorf("oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("oembed status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return meta, fmt.Errorf("read oembed body: %w", err)
	}

	var payload oembedResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return meta, fmt.Errorf("decode oembed: %w", err)
	}

	meta.Title = payload.Title
	meta.ThumbnailURL = payload.ThumbnailURL
	return meta, nil
}

// CompositeResolver выбирает резолвер по провайдеру.
type CompositeResolver struct {
	YouTube *YouTubeResolver
}

func NewCompositeResolver(yt *YouTubeResolver) *CompositeResolver {
	return &CompositeResolver{YouTube: yt}
}

func (c *CompositeResolver) Resolve(ctx context.Context, p ParsedURL) (domain.VideoMeta, error) {
	switch p.Provider {
	case domain.ProviderYouTube:
		if c.YouTube == nil {
			break
		}
		return c.YouTube.Resolve(ctx, p)
	}
	return domain.VideoMeta{
		Provider:   p.Provider,
		ExternalID: p.ExternalID,
	}, nil
}
