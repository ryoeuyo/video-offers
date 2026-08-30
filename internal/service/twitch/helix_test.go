package twitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHelixClient_GetFollowedAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/channels/followers" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"followed_at":"2024-01-02T15:04:05Z"}]}`))
	}))
	defer srv.Close()

	c := NewHelixClient("cid", srv.Client())
	c.baseURL = srv.URL

	at, following, err := c.GetFollowedAt(context.Background(), "tok", "b1", "u1")
	if err != nil {
		t.Fatalf("GetFollowedAt: %v", err)
	}
	if !following {
		t.Fatal("expected following")
	}
	want := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	if !at.Equal(want) {
		t.Errorf("followed_at = %s, want %s", at, want)
	}
}

func TestHelixClient_IsSubscribed_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewHelixClient("cid", srv.Client())
	c.baseURL = srv.URL

	ok, err := c.IsSubscribed(context.Background(), "tok", "b1", "u1")
	if err != nil {
		t.Fatalf("IsSubscribed: %v", err)
	}
	if ok {
		t.Fatal("expected not subscribed")
	}
}
