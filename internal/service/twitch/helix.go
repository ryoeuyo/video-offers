package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultHelixURL = "https://api.twitch.tv/helix"

type HelixAPI interface {
	GetFollowedAt(ctx context.Context, accessToken, broadcasterID, userID string) (followedAt time.Time, following bool, err error)
	IsSubscribed(ctx context.Context, streamerAccessToken, broadcasterID, userID string) (bool, error)
}

type HelixClient struct {
	clientID string
	baseURL  string
	client   HTTPClient
}

func NewHelixClient(clientID string, client HTTPClient) *HelixClient {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &HelixClient{clientID: clientID, baseURL: defaultHelixURL, client: client}
}

func (c *HelixClient) GetFollowedAt(ctx context.Context, accessToken, broadcasterID, userID string) (time.Time, bool, error) {
	q := url.Values{}
	q.Set("broadcaster_id", broadcasterID)
	q.Set("user_id", userID)
	body, status, err := c.get(ctx, "/channels/followers?"+q.Encode(), accessToken)
	if err != nil {
		return time.Time{}, false, err
	}
	if status == http.StatusUnauthorized || status >= 500 {
		return time.Time{}, false, fmt.Errorf("helix followers status %d: %s", status, string(body))
	}
	if status != http.StatusOK {
		return time.Time{}, false, fmt.Errorf("helix followers status %d: %s", status, string(body))
	}

	var payload struct {
		Data []struct {
			FollowedAt time.Time `json:"followed_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return time.Time{}, false, fmt.Errorf("decode followers: %w", err)
	}
	if len(payload.Data) == 0 {
		return time.Time{}, false, nil
	}
	return payload.Data[0].FollowedAt, true, nil
}

func (c *HelixClient) IsSubscribed(ctx context.Context, streamerAccessToken, broadcasterID, userID string) (bool, error) {
	q := url.Values{}
	q.Set("broadcaster_id", broadcasterID)
	q.Set("user_id", userID)
	body, status, err := c.get(ctx, "/subscriptions/user?"+q.Encode(), streamerAccessToken)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("helix subscriptions status %d: %s", status, string(body))
	}

	var payload struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, fmt.Errorf("decode subscriptions: %w", err)
	}
	return len(payload.Data) > 0, nil
}

func (c *HelixClient) get(ctx context.Context, path, accessToken string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", c.clientID)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("helix request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}
