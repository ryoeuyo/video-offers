package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ruslan/video-offers/internal/domain"
)

func TestIssueAndParseAccess(t *testing.T) {
	svc := New("test-secret-key-at-least-32-bytes-long", 15*time.Minute)
	userID := uuid.Must(uuid.NewV7())

	token, exp, err := svc.IssueAccess(userID, domain.RoleStreamer)
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if exp.Before(time.Now()) {
		t.Error("exp should be in the future")
	}

	gotID, gotRole, err := svc.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if gotID != userID {
		t.Errorf("user id = %v, want %v", gotID, userID)
	}
	if gotRole != domain.RoleStreamer {
		t.Errorf("role = %q, want streamer", gotRole)
	}
}

func TestParseAccess_Invalid(t *testing.T) {
	svc := New("test-secret-key-at-least-32-bytes-long", 15*time.Minute)

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "garbage", token: "not.a.jwt"},
		{name: "wrong secret", token: mustToken(t, "other-secret-key-at-least-32-bytes-long")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.ParseAccess(tt.token)
			if err == nil {
				t.Fatal("expected error")
			}
			domErr, ok := domain.AsError(err)
			if !ok {
				t.Fatalf("expected domain error, got %v", err)
			}
			if domErr.Code != "invalid_token" {
				t.Errorf("code = %q, want invalid_token", domErr.Code)
			}
		})
	}
}

func TestParseAccess_Expired(t *testing.T) {
	svc := New("test-secret-key-at-least-32-bytes-long", -time.Second)
	userID := uuid.Must(uuid.NewV7())

	token, _, err := svc.IssueAccess(userID, domain.RoleViewer)
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}

	_, _, err = svc.ParseAccess(token)
	if err == nil {
		t.Fatal("expected expired token error")
	}
}

func mustToken(t *testing.T, secret string) string {
	t.Helper()
	other := New(secret, 15*time.Minute)
	tok, _, err := other.IssueAccess(uuid.Must(uuid.NewV7()), domain.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
