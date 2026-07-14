// Package upload stores user-uploaded files on the local filesystem.
package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lzqqdy/marketpulse/internal/config"
)

var allowedAvatar = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Store saves uploaded assets under cfg.Dir and exposes them via cfg.PublicPath.
type Store struct {
	cfg config.UploadConfig
	dir string
}

// New ensures the upload directory exists.
func New(cfg config.UploadConfig) (*Store, error) {
	abs, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("upload dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, "avatars"), 0o755); err != nil {
		return nil, fmt.Errorf("upload mkdir: %w", err)
	}
	return &Store{cfg: cfg, dir: abs}, nil
}

// Dir returns the absolute filesystem root for Static mounting.
func (s *Store) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

// PublicPath returns the URL prefix (e.g. /uploads).
func (s *Store) PublicPath() string {
	if s == nil {
		return "/uploads"
	}
	p := strings.TrimRight(s.cfg.PublicPath, "/")
	if p == "" {
		return "/uploads"
	}
	return p
}

// SaveAvatar writes an image under avatars/ and returns a public URL path.
func (s *Store) SaveAvatar(userID int64, fh *multipart.FileHeader) (publicURL string, err error) {
	if s == nil {
		return "", fmt.Errorf("upload: store unavailable")
	}
	if fh == nil {
		return "", fmt.Errorf("upload: empty file")
	}
	if fh.Size > 0 && fh.Size > s.cfg.MaxAvatarBytes {
		return "", fmt.Errorf("upload: file too large (max %d bytes)", s.cfg.MaxAvatarBytes)
	}

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("upload open: %w", err)
	}
	defer src.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	contentType := http.DetectContentType(head[:n])
	ext, ok := allowedAvatar[contentType]
	if !ok {
		return "", fmt.Errorf("upload: unsupported image type %s", contentType)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		// multipart temp files may not seek; reopen
		_ = src.Close()
		src, err = fh.Open()
		if err != nil {
			return "", fmt.Errorf("upload reopen: %w", err)
		}
		defer src.Close()
	}

	id, err := randomID()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d_%s%s", userID, id, ext)
	rel := filepath.Join("avatars", name)
	dstPath := filepath.Join(s.dir, rel)
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("upload create: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, io.LimitReader(src, s.cfg.MaxAvatarBytes+1))
	if err != nil {
		_ = os.Remove(dstPath)
		return "", fmt.Errorf("upload write: %w", err)
	}
	if written > s.cfg.MaxAvatarBytes {
		_ = os.Remove(dstPath)
		return "", fmt.Errorf("upload: file too large (max %d bytes)", s.cfg.MaxAvatarBytes)
	}

	publicURL = s.PublicPath() + "/avatars/" + name
	return publicURL, nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("upload id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
