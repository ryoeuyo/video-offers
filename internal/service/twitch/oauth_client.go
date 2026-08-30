package twitch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAuthURL  = "https://id.twitch.tv/oauth2/authorize"
	defaultTokenURL = "https://id.twitch.tv/oauth2/token"
	defaultAPIURL   = "https://api.twitch.tv/helix/users"
)

// DefaultScopes — follow для отправителей, subscriptions для sub-gate стримера.
const DefaultScopes = "user:read:follows channel:read:subscriptions"

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type OAuthClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	authURL      string
	tokenURL     string
	apiURL       string
	client       HTTPClient
}

func NewOAuthClient(clientID, clientSecret, redirectURI string, client HTTPClient) *OAuthClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &OAuthClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		authURL:      defaultAuthURL,
		tokenURL:     defaultTokenURL,
		apiURL:       defaultAPIURL,
		client:       client,
	}
}

type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type UserResponse struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

func NewCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (c *OAuthClient) AuthorizeURL(state, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", DefaultScopes)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	return c.authURL + "?" + q.Encode()
}

func (c *OAuthClient) ExchangeCode(ctx context.Context, code, codeVerifier string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.redirectURI)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TokenResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("token status %d: %s", resp.StatusCode, string(body))
	}

	var out TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return TokenResponse{}, fmt.Errorf("decode token: %w", err)
	}
	return out, nil
}

func (c *OAuthClient) GetCurrentUser(ctx context.Context, accessToken string) (UserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return UserResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", c.clientID)

	resp, err := c.client.Do(req)
	if err != nil {
		return UserResponse{}, fmt.Errorf("users request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return UserResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return UserResponse{}, fmt.Errorf("users status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Data []UserResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return UserResponse{}, fmt.Errorf("decode users: %w", err)
	}
	if len(payload.Data) == 0 {
		return UserResponse{}, fmt.Errorf("empty users response")
	}
	return payload.Data[0], nil
}
