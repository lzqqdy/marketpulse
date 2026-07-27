package admin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrForbidden = errors.New("admin: forbidden")
	ErrInvalid   = errors.New("admin: invalid config")
)

// Service manages admin identity and config file editing.
type Service struct {
	adminFile   string
	configPath  string
	examplePath string
}

// New creates an admin service. Paths are relative to process CWD unless absolute.
func New(adminFile, configPath, examplePath string) *Service {
	if strings.TrimSpace(adminFile) == "" {
		adminFile = "admin.txt"
	}
	if strings.TrimSpace(configPath) == "" {
		configPath = "config/config.yaml"
	}
	if strings.TrimSpace(examplePath) == "" {
		examplePath = DefaultExamplePath(configPath)
	}
	return &Service{
		adminFile:   adminFile,
		configPath:  configPath,
		examplePath: examplePath,
	}
}

// DefaultExamplePath derives config.example.yaml next to the live config file.
func DefaultExamplePath(configPath string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, "config.example.yaml")
}

// IsAdmin reports whether phone matches admin.txt (file missing/empty => false).
func (s *Service) IsAdmin(phone string) bool {
	want, err := s.readAdminPhone()
	if err != nil || want == "" {
		return false
	}
	return normalizePhone(phone) == want
}

func (s *Service) readAdminPhone() (string, error) {
	data, err := os.ReadFile(s.adminFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return normalizePhone(line), nil
	}
	return "", nil
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	return phone
}

// RequireAdmin returns ErrForbidden when phone is not admin.
func (s *Service) RequireAdmin(phone string) error {
	if !s.IsAdmin(phone) {
		return ErrForbidden
	}
	return nil
}

// ConfigPaths returns live/example paths for API responses.
func (s *Service) ConfigPaths() (live, example string) {
	return s.configPath, s.examplePath
}

func readFileOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", err
	}
	return string(data), nil
}
