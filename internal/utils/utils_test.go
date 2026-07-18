package utils_test

import (
	"testing"

	"github.com/pulse/chat-service/internal/utils"
)

func TestHashPasswordAndCheck(t *testing.T) {
	hash, err := utils.HashPassword("secret12345")
	if err != nil {
		t.Fatal(err)
	}
	if !utils.CheckPassword(hash, "secret12345") {
		t.Fatal("expected password match")
	}
	if utils.CheckPassword(hash, "wrong") {
		t.Fatal("expected mismatch")
	}
}

func TestExtractMentions(t *testing.T) {
	got := utils.ExtractMentions("hello @alice and @bob and @alice")
	if len(got) != 2 {
		t.Fatalf("expected 2 mentions, got %v", got)
	}
}

func TestSanitizeText(t *testing.T) {
	s := utils.SanitizeText("  hello\x00world  ", 5)
	if s != "hello" {
		t.Fatalf("unexpected: %q", s)
	}
}
