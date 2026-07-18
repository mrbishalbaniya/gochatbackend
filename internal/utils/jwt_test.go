package utils_test

import (
	"testing"
	"time"

	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	secret := "test-access-secret-min-32-characters!"
	uid := uuid.New()
	token, _, err := utils.IssueAccessToken(secret, time.Minute, uid, "a@b.com", "alice", "user")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := utils.ParseAccessToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != uid {
		t.Fatalf("uid mismatch")
	}
}
