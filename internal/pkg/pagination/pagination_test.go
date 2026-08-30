package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncodeDecode(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	createdAt := time.Date(2026, 1, 15, 12, 0, 0, 123456789, time.UTC)
	key := Key{CreatedAt: createdAt, ID: id}

	raw := Encode(key)
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("created_at = %v, want %v", got.CreatedAt, createdAt)
	}
	if got.ID != id {
		t.Errorf("id = %v, want %v", got.ID, id)
	}
}

func TestDecode_Invalid(t *testing.T) {
	tests := []string{"", "!!!", "not-valid-base64"}
	for _, raw := range tests {
		if _, err := Decode(raw); err == nil {
			t.Errorf("Decode(%q) expected error", raw)
		}
	}
}

func TestClampLimit(t *testing.T) {
	if got := ClampLimit(0, 20, 100); got != 20 {
		t.Errorf("got %d, want 20", got)
	}
	if got := ClampLimit(50, 20, 100); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
	if got := ClampLimit(200, 20, 100); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
}
