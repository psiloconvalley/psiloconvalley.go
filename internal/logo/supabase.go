package logo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// SupabaseStore uploads logos to Supabase Storage via the S3-compatible REST API.
// Required env vars:
//
//	SUPABASE_URL                e.g. https://xxxx.supabase.co
//	SUPABASE_SERVICE_ROLE_KEY   service_role secret key (privileged, server-only)
//	SUPABASE_STORAGE_BUCKET     e.g. logos
type SupabaseStore struct {
	baseURL    string // https://xxxx.supabase.co
	serviceKey string // service_role key
	bucket     string // bucket name
	httpClient *http.Client
}

// NewSupabaseStore validates required env vars and returns a configured store.
// Returns an error if any required env var is missing or empty — caller should
// treat this as a fatal boot error, not a runtime fallback. Silent fallback to
// local storage in production would silently lose user data on the next deploy
// (Railway's ephemeral disk).
func NewSupabaseStore() (*SupabaseStore, error) {
	base := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	bucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	if base == "" || key == "" || bucket == "" {
		return nil, fmt.Errorf(
			"supabase: missing required env vars (need all of: SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, SUPABASE_STORAGE_BUCKET)",
		)
	}
	return &SupabaseStore{
		baseURL:    base,
		serviceKey: key,
		bucket:     bucket,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// objectPath returns the stable storage path for a user's logo.
func (s *SupabaseStore) objectPath(userID int64) string {
	return fmt.Sprintf("logo-user-%d.png", userID)
}

// publicURL returns the Supabase public URL for a stored object.
func (s *SupabaseStore) publicURL(userID int64) string {
	return fmt.Sprintf(
		"%s/storage/v1/object/public/%s/%s",
		s.baseURL,
		s.bucket,
		s.objectPath(userID),
	)
}

// Save uploads PNG bytes to Supabase Storage and returns the public URL.
// Uses upsert (overwrite) so re-uploads replace the previous logo cleanly.
func (s *SupabaseStore) Save(userID int64, data []byte) (string, error) {
	url := fmt.Sprintf(
		"%s/storage/v1/object/%s/%s",
		s.baseURL,
		s.bucket,
		s.objectPath(userID),
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("supabase: build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("x-upsert", "true") // overwrite on re-upload

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("supabase: upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("supabase: upload failed (%d): %s", resp.StatusCode, body)
	}

	return s.publicURL(userID), nil
}

// Delete removes a user's logo from Supabase Storage.
func (s *SupabaseStore) Delete(userID int64) error {
	url := fmt.Sprintf(
		"%s/storage/v1/object/%s/%s",
		s.baseURL,
		s.bucket,
		s.objectPath(userID),
	)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("supabase: build delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.serviceKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("supabase: delete request: %w", err)
	}
	defer resp.Body.Close()

	// 200 or 404 are both acceptable — 404 means already gone
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase: delete failed (%d): %s", resp.StatusCode, body)
	}

	return nil
}
