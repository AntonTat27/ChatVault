package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	whisperURL = "https://api.openai.com/v1/audio/transcriptions"
)

// WhisperClient transcribes Telegram voice messages via OpenAI Whisper.
type WhisperClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

// NewWhisperClient creates a WhisperClient with configured timeout.
func NewWhisperClient(apiKey string, model string, timeout time.Duration) *WhisperClient {
	return &WhisperClient{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		model:      model,
	}
}

// Transcribe transcribes OGG bytes with one retry and backoff.
func (w *WhisperClient) Transcribe(ctx context.Context, fileName string, content []byte) (string, error) {
	if w.apiKey == "" {
		return "", fmt.Errorf("openai api key is not configured")
	}
	var lastErr error
	for attempt := 1; attempt <= maxAITryCount; attempt++ {
		text, err := w.doTranscribe(ctx, fileName, content)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if attempt < maxAITryCount {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(aiRetryDelay):
			}
		}
	}
	return "", fmt.Errorf("whisper request failed after retry: %w", lastErr)
}

// doTranscribe performs one transcription request.
func (w *WhisperClient) doTranscribe(ctx context.Context, fileName string, content []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("model", w.model); err != nil {
		return "", err
	}
	fileField, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := fileField.Write(content); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+w.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("whisper status %d", resp.StatusCode)
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", err
	}
	if out.Text == "" {
		return "", fmt.Errorf("whisper empty transcript")
	}
	return out.Text, nil
}
