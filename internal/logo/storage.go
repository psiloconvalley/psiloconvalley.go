// internal/logo/storage.go
package logo

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store defines the logo storage interface.
// Implementations must be swappable without changing any handler code.
//
// Contract:
//   - Save returns a URL suitable for storage in the database.
//   - LocalStore always returns a root-relative path (/static/...).
//   - SupabaseStore will return an absolute https:// URL.
//   - Callers that need an absolute URL (e.g. email links) must
//     prepend APP_BASE_URL themselves — storage is not responsible
//     for knowing the deployment host.
type Store interface {
	Save(userID int64, data []byte) (publicURL string, err error)
	Delete(userID int64) error
}

// =====================================================================
// LocalStore — disk-backed implementation
//
// Used in development and current production (Railway ephemeral disk).
// Files are written to BaseDir and served by the Go static file handler.
//
// BaseDir: relative path from binary working directory, e.g. "static/uploads/logos"
// BaseURL: retained on the struct for future use (e.g. SupabaseStore needs it).
//          LocalStore does NOT use BaseURL — it always returns a relative path.
//          Rationale: the PDF handler reads the file from disk using the relative
//          path. If we stored an absolute URL, the PDF handler would try to fetch
//          it over the network from the deployment host — which fails locally and
//          is unnecessary overhead in production.
// =====================================================================

type LocalStore struct {
	BaseDir string // e.g. "static/uploads/logos"
	BaseURL string // reserved — not used by LocalStore
}

// NewLocalStore returns a LocalStore rooted at baseDir.
// baseURL is accepted for interface consistency but is not used.
func NewLocalStore(baseDir, baseURL string) *LocalStore {
	return &LocalStore{
		BaseDir: baseDir,
		BaseURL: baseURL,
	}
}

// Save writes the processed logo PNG to disk and returns a root-relative URL.
// The filename is stable per user — a new upload silently replaces the old one.
// Returned URL format: /static/uploads/logos/logo-user-{id}.png
func (s *LocalStore) Save(userID int64, data []byte) (string, error) {
	if err := os.MkdirAll(s.BaseDir, 0755); err != nil {
		return "", fmt.Errorf("logo: could not create directory %q: %w", s.BaseDir, err)
	}

	filename := fmt.Sprintf("logo-user-%d.png", userID)
	path := filepath.Join(s.BaseDir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("logo: could not write %q: %w", path, err)
	}

	// Always root-relative. The PDF handler reads from disk via this path.
	// Email handlers that need an absolute URL prepend APP_BASE_URL themselves.
	return "/static/uploads/logos/" + filename, nil
}

// Delete removes the logo file for the given user.
// Returns nil if the file does not exist — idempotent by design.
func (s *LocalStore) Delete(userID int64) error {
	filename := fmt.Sprintf("logo-user-%d.png", userID)
	path := filepath.Join(s.BaseDir, filename)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logo: could not delete %q: %w", path, err)
	}
	return nil
}
