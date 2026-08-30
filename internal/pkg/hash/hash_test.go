package hash

import (
	"errors"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	encoded, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if encoded == "" {
		t.Fatal("empty hash")
	}

	if err := Verify("correct horse battery", encoded); err != nil {
		t.Errorf("Verify correct password: %v", err)
	}
	if err := Verify("wrong password", encoded); !errors.Is(err, ErrMismatch) {
		t.Errorf("Verify wrong password: got %v, want ErrMismatch", err)
	}
}

func TestVerify_InvalidFormat(t *testing.T) {
	err := Verify("password", "not-a-valid-hash")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if errors.Is(err, ErrMismatch) {
		t.Error("should not be ErrMismatch")
	}
}
