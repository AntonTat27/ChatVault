package supabase

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
)

// StorageClient uploads voice files to Supabase Storage.
type StorageClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	bucket     string
}

// NewStorageClient creates a storage client with timeout and credentials.
func NewStorageClient(baseURL string, apiKey string, bucket string, timeout time.Duration) *StorageClient {
	return &StorageClient{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		apiKey:     apiKey,
		bucket:     bucket,
	}
}

// UploadVoice uploads a voice OGG payload to Supabase Storage.
func (c *StorageClient) UploadVoice(ctx context.Context, storagePath string, payload []byte) error {
	if c.baseURL == "" || c.apiKey == "" {
		return fmt.Errorf("supabase storage configuration is missing")
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, "/storage/v1/object", c.bucket, storagePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "audio/ogg")
	req.Header.Set("x-upsert", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("supabase storage status %d", resp.StatusCode)
	}
	return nil
}
