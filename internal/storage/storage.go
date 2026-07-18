package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
)

type LocalStorage struct {
	Root        string
	PublicBase  string
	MaxBytes    int64
}

type SavedFile struct {
	FileName     string
	MimeType     string
	SizeBytes    int64
	StoragePath  string
	URL          string
	Checksum     string
	ThumbnailURL string
}

func (s *LocalStorage) Save(uploaderID uuid.UUID, originalName string, reader io.Reader, size int64, contentType string) (*SavedFile, error) {
	if size <= 0 || size > s.MaxBytes {
		return nil, fmt.Errorf("file size exceeds limit")
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(originalName))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !utils.AllowedMIME(contentType) {
		return nil, fmt.Errorf("unsupported mime type: %s", contentType)
	}

	ext := strings.ToLower(filepath.Ext(originalName))
	day := time.Now().UTC().Format("2006/01/02")
	dir := filepath.Join(s.Root, day, uploaderID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	safeName := sanitizeFileName(originalName)
	stored := id + ext
	fullPath := filepath.Join(dir, stored)

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	written, err := io.Copy(mw, io.LimitReader(reader, s.MaxBytes+1))
	if err != nil {
		_ = os.Remove(fullPath)
		return nil, err
	}
	if written > s.MaxBytes {
		_ = os.Remove(fullPath)
		return nil, fmt.Errorf("file size exceeds limit")
	}

	rel := filepath.ToSlash(filepath.Join(day, uploaderID.String(), stored))
	url := strings.TrimRight(s.PublicBase, "/") + "/uploads/" + rel

	return &SavedFile{
		FileName:    safeName,
		MimeType:    contentType,
		SizeBytes:   written,
		StoragePath: fullPath,
		URL:         url,
		Checksum:    hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// VirusScanHook is a no-op hook point for integrating ClamAV or similar scanners.
func VirusScanHook(path string) error {
	_ = path
	return nil
}
