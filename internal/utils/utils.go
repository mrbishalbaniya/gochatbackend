package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]{3,32})`)

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

func SanitizeText(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	if max > 0 && utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max])
	}
	return s
}

func ExtractMentions(body string) []string {
	matches := mentionRegex.FindAllStringSubmatch(body, -1)
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		u := strings.ToLower(m[1])
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

func AllowedMIME(mime string) bool {
	allowed := map[string]struct{}{
		"image/jpeg": {}, "image/png": {}, "image/gif": {}, "image/webp": {},
		"video/mp4": {}, "video/webm": {},
		"audio/mpeg": {}, "audio/mp4": {}, "audio/ogg": {}, "audio/webm": {}, "audio/wav": {},
		"application/pdf": {},
		"application/msword": {},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
		"application/vnd.ms-excel": {},
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
		"application/vnd.ms-powerpoint": {},
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
		"application/zip": {}, "application/x-rar-compressed": {}, "application/vnd.rar": {},
		"application/vnd.android.package-archive": {},
		"application/octet-stream": {},
	}
	_, ok := allowed[mime]
	return ok
}
