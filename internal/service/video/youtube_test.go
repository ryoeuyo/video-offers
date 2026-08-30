package video

import "testing"

func TestYouTubeThumbnailURL(t *testing.T) {
	got := YouTubeThumbnailURL("dQw4w9WgXcQ")
	want := "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if YouTubeThumbnailURL("bad") != "" {
		t.Error("expected empty for invalid id")
	}
}
