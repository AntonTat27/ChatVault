//go:build ignore
// +build ignore

package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	geminiTranscribeMaxTokens = 1024
)

// GeminiTranscribeClient transcribes Telegram voice messages via Gemini.
type GeminiTranscribeClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

// NewGeminiTranscribeClient creates a GeminiTranscribeClient with configured timeout.
func NewGeminiTranscribeClient(apiKey string, model string, timeout time.Duration) *GeminiTranscribeClient {
	return &GeminiTranscribeClient{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		model:      model,
	}
}

// Transcribe transcribes OGG bytes with retry and backoff.
func (w *GeminiTranscribeClient) Transcribe(ctx context.Context, fileName string, content []byte) (string, error) {
	if w.apiKey == "" {
		return "", fmt.Errorf("gemini api key is not configured")
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
	return "", fmt.Errorf("gemini transcription failed after retry: %w", lastErr)
}

// doTranscribe performs one transcription request.
func (w *GeminiTranscribeClient) doTranscribe(ctx context.Context, fileName string, content []byte) (string, error) {
	encodedAudio := base64.StdEncoding.EncodeToString(content)
	request := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: fmt.Sprintf("Transcribe the following audio. Return only the transcript text. File: %s", fileName)},
					{InlineData: &geminiInlineData{MimeType: "audio/ogg", Data: encodedAudio}},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens:  geminiTranscribeMaxTokens,
			Temperature:      geminiTemperature,
			ResponseMimeType: geminiResponseText,
		},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	return doGeminiRequest(ctx, w.httpClient, w.apiKey, w.model, body)
}
