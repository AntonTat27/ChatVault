package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// embeddingMaxTryCount mirrors the retry policy used for classification/summary
// requests, since the embedding endpoint is subject to the same transient
// failures (rate limits, timeouts).
const embeddingMaxTryCount = maxAITryCount

// GenerateEmbedding computes a vector embedding for the given text using the
// configured Gemini embedding model.
func (c *GeminiClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("gemini api key is not configured")
	}
	if c.embedModel == "" {
		return nil, fmt.Errorf("gemini embedding model is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= embeddingMaxTryCount; attempt++ {
		values, err := c.doEmbed(ctx, text)
		if err == nil {
			return values, nil
		}
		lastErr = err
		if attempt < embeddingMaxTryCount {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(aiRetryDelay):
			}
		}
	}
	return nil, fmt.Errorf("gemini embedding request failed after retry: %w", lastErr)
}

func (c *GeminiClient) doEmbed(ctx context.Context, text string) ([]float32, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(c.apiKey), option.WithHTTPClient(c.httpClient))
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	embeddingModel := client.EmbeddingModel(c.embedModel)
	resp, err := embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Embedding == nil || len(resp.Embedding.Values) == 0 {
		return nil, fmt.Errorf("gemini embedding response had no values")
	}
	return resp.Embedding.Values, nil
}
