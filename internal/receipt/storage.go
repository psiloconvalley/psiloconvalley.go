package receipt

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MaxFileSize = 5 << 20 // 5MB

// ValidateFile checks content type is image or PDF.
// Returns the file extension to use for storage.
func ValidateFile(data []byte, originalName string) (ext string, err error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty file")
	}
	if len(data) > MaxFileSize {
		return "", fmt.Errorf("file too large (max 5MB)")
	}

	contentType := http.DetectContentType(data)
	switch {
	case contentType == "image/jpeg":
		return ".jpg", nil
	case contentType == "image/png":
		return ".png", nil
	case contentType == "image/webp":
		return ".webp", nil
	case contentType == "application/pdf" ||
		strings.HasSuffix(strings.ToLower(originalName), ".pdf"):
		return ".pdf", nil
	default:
		return "", fmt.Errorf("unsupported file type: %s (must be image or PDF)", contentType)
	}
}

// Store defines the receipt storage interface.
type Store interface {
	Save(userID, expenseID int64, data []byte, ext string) (publicURL string, err error)
	Delete(userID, expenseID int64) error
}

// =====================================================================
// LocalStore — disk-backed implementation for development
// =====================================================================

type LocalStore struct {
	BaseDir string
	BaseURL string
}

func NewLocalStore(baseDir, baseURL string) *LocalStore {
	return &LocalStore{BaseDir: baseDir, BaseURL: baseURL}
}

func (s *LocalStore) filename(userID, expenseID int64, ext string) string {
	return fmt.Sprintf("receipt-%d-%d%s", userID, expenseID, ext)
}

func (s *LocalStore) Save(userID, expenseID int64, data []byte, ext string) (string, error) {
	if err := os.MkdirAll(s.BaseDir, 0755); err != nil {
		return "", fmt.Errorf("receipt: create dir: %w", err)
	}

	name := s.filename(userID, expenseID, ext)
	path := filepath.Join(s.BaseDir, name)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("receipt: write file: %w", err)
	}

	return "/static/uploads/receipts/" + name, nil
}

func (s *LocalStore) Delete(userID, expenseID int64) error {
	// Try all known extensions
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".pdf"} {
		name := s.filename(userID, expenseID, ext)
		path := filepath.Join(s.BaseDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("receipt: delete: %w", err)
		}
	}
	return nil
}
