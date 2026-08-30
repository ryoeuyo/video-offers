package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Key — составной курсор (created_at, id) для стабильной пагинации.
type Key struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func Encode(k Key) string {
	payload := k.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + k.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func Decode(raw string) (Key, error) {
	if raw == "" {
		return Key{}, fmt.Errorf("empty cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Key{}, fmt.Errorf("decode cursor: %w", err)
	}

	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return Key{}, fmt.Errorf("invalid cursor format")
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Key{}, fmt.Errorf("parse cursor time: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return Key{}, fmt.Errorf("parse cursor id: %w", err)
	}

	return Key{CreatedAt: createdAt, ID: id}, nil
}

// ClampLimit нормализует limit: default если ≤0, cap на max.
func ClampLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
