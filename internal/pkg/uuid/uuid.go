package uuid

import (
	"fmt"

	"github.com/google/uuid"
)

// New генерирует UUID v7 — монотонный по времени, удобен для cursor-пагинации.
func New() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate uuid v7: %w", err)
	}
	return id, nil
}
