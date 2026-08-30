package seal

import "testing"

func TestSealOpen(t *testing.T) {
	key := "test-key-for-seal"
	plain := "refresh_token_secret_value"

	blob, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	got, err := Open(key, blob)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != plain {
		t.Errorf("got %q, want %q", got, plain)
	}
}

func TestEncodeDecodeState(t *testing.T) {
	payload := []byte(`{"user_id":"abc","verifier":"xyz"}`)
	encoded, err := EncodeState("state-key", payload)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	got, err := DecodeState("state-key", encoded)
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("got %s, want %s", got, payload)
	}
}
