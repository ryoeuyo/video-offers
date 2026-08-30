package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruslan/video-offers/internal/domain"
)

func TestParseURL_YouTubeNormalization(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		normalized string
		externalID string
	}{
		{
			name:       "watch",
			raw:        "https://www.youtube.com/watch?v=dQw4w9WgXcQ&utm_source=tg",
			normalized: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			externalID: "dQw4w9WgXcQ",
		},
		{
			name:       "youtu.be",
			raw:        "https://youtu.be/dQw4w9WgXcQ",
			normalized: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			externalID: "dQw4w9WgXcQ",
		},
		{
			name:       "shorts",
			raw:        "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			normalized: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			externalID: "dQw4w9WgXcQ",
		},
		{
			name:       "embed",
			raw:        "https://www.youtube.com/embed/dQw4w9WgXcQ",
			normalized: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			externalID: "dQw4w9WgXcQ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.raw)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			if got.Normalized != tt.normalized {
				t.Errorf("normalized = %q, want %q", got.Normalized, tt.normalized)
			}
			if got.ExternalID != tt.externalID {
				t.Errorf("external_id = %q, want %q", got.ExternalID, tt.externalID)
			}
			if got.Provider != domain.ProviderYouTube {
				t.Errorf("provider = %q, want youtube", got.Provider)
			}
		})
	}
}

func TestParseURL_SSRF(t *testing.T) {
	tests := []string{
		"http://127.0.0.1/video",
		"http://localhost/secret",
		"http://192.168.1.1/",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseURL(raw)
			if err == nil {
				t.Fatal("expected SSRF error")
			}
			domErr, ok := domain.AsError(err)
			if !ok || domErr.Kind != domain.KindUnprocessable {
				t.Fatalf("expected unprocessable, got %v", err)
			}
		})
	}
}

func TestParseURL_OtherProvider(t *testing.T) {
	got, err := ParseURL("https://example.com/video/1?utm_source=x")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got.Provider != domain.ProviderOther {
		t.Errorf("provider = %q, want other", got.Provider)
	}
	if got.Normalized == "" {
		t.Error("expected normalized url")
	}
}

func TestParseURL_Invalid(t *testing.T) {
	_, err := ParseURL("ftp://youtube.com/watch?v=dQw4w9WgXcQ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestYouTubeResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format = %q", r.URL.Query().Get("format"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"title":         "Test Video",
			"thumbnail_url": "https://i.ytimg.com/vi/abc/hqdefault.jpg",
		})
	}))
	defer srv.Close()

	resolver := &YouTubeResolver{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	meta, err := resolver.Resolve(context.Background(), ParsedURL{
		Normalized: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		ExternalID: "dQw4w9WgXcQ",
		Provider:   domain.ProviderYouTube,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if meta.Title != "Test Video" {
		t.Errorf("title = %q", meta.Title)
	}
	if meta.ThumbnailURL == "" {
		t.Error("expected thumbnail")
	}
}

func TestCompositeResolver_NonYouTube(t *testing.T) {
	resolver := NewCompositeResolver(NewYouTubeResolver(nil))
	meta, err := resolver.Resolve(context.Background(), ParsedURL{
		Provider: domain.ProviderOther,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if meta.Provider != domain.ProviderOther {
		t.Errorf("provider = %q", meta.Provider)
	}
}
