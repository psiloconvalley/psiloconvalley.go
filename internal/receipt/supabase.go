package receipt

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// SupabaseStore uploads receipts to Supabase Storage.
// Uses the same bucket as logos but under a different path prefix.
// Required env vars: SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, SUPABASE_STORAGE_BUCKET
type SupabaseStore struct {
	baseURL    string
	serviceKey string
	bucket     string
	httpClient *http.Client
}

func NewSupabaseStore() (*SupabaseStore, error) {
	base := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	bucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	if base == "" || key == "" || bucket == "" {
		return nil, fmt.Errorf(
			"receipt: missing required env vars (SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, SUPABASE_STORAGE_BUCKET)",
		)
	}
	return &SupabaseStore{
		baseURL:    base,
		serviceKey: key,
		bucket:     bucket,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (s *SupabaseStore) objectPath(userID, expenseID int64, ext string) string {
	return fmt.Sprintf("receipts/%d/%d%s", userID, expenseID, ext)
}

func (s *SupabaseStore) publicURL(userID, expenseID int64, ext string) string {
	return fmt.Sprintf(
		"%s/storage/v1/object/public/%s/%s",
		s.baseURL, s.bucket,
		s.objectPath(userID, expenseID, ext),
	)
}

func contentTypeFromExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func (s *SupabaseStore) Save(userID, expenseID int64, data []byte, ext string) (string, error) {
	path := s.objectPath(userID, expenseID, ext)
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, path)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("receipt: build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("Content-Type", contentTypeFromExt(ext))
	req.Header.Set("x-upsert", "true")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("receipt: upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("receipt: upload failed (%d): %s", resp.StatusCode, body)
	}

	return s.publicURL(userID, expenseID, ext), nil
}

func (s *SupabaseStore) Delete(userID, expenseID int64) error {
	// Try all known extensions
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".pdf"} {
		path := s.objectPath(userID, expenseID, ext)
		url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, path)

		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+s.serviceKey)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
	return nil
}
