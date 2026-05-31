//go:build ignore
// +build ignore

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chatvault/internal/model"
)

const (
	geminiBaseURL      = "https://generativelanguage.googleapis.com/v1beta/models/"
	maxAITryCount      = 2
	aiRetryDelay       = 5 * time.Second
	geminiMaxTokens    = 512
	geminiTemperature  = 0.2
	geminiResponseJSON = "application/json"
	geminiResponseText = "text/plain"
)

const classificationPromptTemplate = `You are classifying a message from a professional team group chat.

Classify the message into EXACTLY ONE of these types:
- "idea" — a suggestion, proposal, or creative thought
- "decision" — something the team has agreed on or resolved
- "action-item" — a task assigned to someone or a next step
- "question" — an open question or request for input
- "document" — a file, link, or reference being shared
- "noise" — casual chat, greetings, reactions, off-topic

Also extract a topic in 2-4 words if the message is clearly about a specific subject (e.g., "Q3 marketing plan", "backend deployment", "hiring process"). Return null if no clear topic.

Message: "%s"

Respond ONLY in this JSON format, no other text:
{"type": "idea", "topic": "Q3 marketing plan"}`

const summaryPromptTemplate = `You are summarising a full day of messages from a professional team group chat.

Below are all messages from today, each with their type tag.

Your output must be a structured JSON with these fields:
- "summary": 3-5 sentence narrative overview of the day's discussion
- "decisions": array of strings, each a clear decision that was made
- "action_items": array of objects with "task" and "owner" (owner is null if unassigned)
- "ideas": array of strings, each a notable idea raised
- "open_questions": array of strings, each an unresolved question

Messages:
%s

Respond ONLY with the JSON object. No markdown, no preamble.`

// GeminiClient performs AI classification and summary requests.
type GeminiClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

// NewGeminiClient constructs GeminiClient with request timeout.
func NewGeminiClient(apiKey string, modelName string, timeout time.Duration) *GeminiClient {
	return &GeminiClient{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		model:      modelName,
	}
}

// BuildClassificationPrompt creates the exact classification prompt with message interpolation.
func BuildClassificationPrompt(messageText string) string {
	sanitized := strings.ReplaceAll(messageText, "\"", "\\\"")
	return fmt.Sprintf(classificationPromptTemplate, sanitized)
}

// BuildSummaryPrompt creates the exact daily summary prompt with message payload interpolation.
func BuildSummaryPrompt(messagesJSON string) string {
	return fmt.Sprintf(summaryPromptTemplate, messagesJSON)
}

// ClassifyMessage tags a message using Gemini and parses strict JSON output.
func (c *GeminiClient) ClassifyMessage(ctx context.Context, messageText string) (model.ClassificationResult, error) {
	prompt := BuildClassificationPrompt(messageText)
	raw, err := c.sendPrompt(ctx, prompt)
	if err != nil {
		return model.ClassificationResult{}, err
	}
	return ParseClassificationResult(raw)
}

// GenerateSummary requests the daily summary JSON payload from Gemini.
func (c *GeminiClient) GenerateSummary(ctx context.Context, messagesJSON string) (model.DailySummary, error) {
	prompt := BuildSummaryPrompt(messagesJSON)
	raw, err := c.sendPrompt(ctx, prompt)
	if err != nil {
		return model.DailySummary{}, err
	}

	var result model.DailySummary
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return model.DailySummary{}, fmt.Errorf("summary parse failed: %w", err)
	}
	return result, nil
}

// ParseClassificationResult parses and validates classification JSON.
func ParseClassificationResult(raw string) (model.ClassificationResult, error) {
	candidate := strings.TrimSpace(raw)
	if idx := strings.Index(candidate, "{"); idx > 0 {
		candidate = candidate[idx:]
	}
	if end := strings.LastIndex(candidate, "}"); end >= 0 {
		candidate = candidate[:end+1]
	}

	var result model.ClassificationResult
	if err := json.Unmarshal([]byte(candidate), &result); err != nil {
		return model.ClassificationResult{}, fmt.Errorf("classification parse failed: %w", err)
	}
	if _, ok := model.AllowedAITags[result.Type]; !ok {
		return model.ClassificationResult{}, fmt.Errorf("invalid classification type: %s", result.Type)
	}
	if result.Topic != nil {
		trimmed := strings.TrimSpace(*result.Topic)
		if trimmed == "" || strings.EqualFold(trimmed, "null") {
			result.Topic = nil
		} else {
			result.Topic = &trimmed
		}
	}
	return result, nil
}

// sendPrompt executes a Gemini request with retry/backoff policy.
func (c *GeminiClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("gemini api key is not configured")
	}

	request := geminiRequest{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: prompt}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens:  geminiMaxTokens,
			Temperature:      geminiTemperature,
			ResponseMimeType: geminiResponseJSON,
		},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 1; attempt <= maxAITryCount; attempt++ {
		result, reqErr := doGeminiRequest(ctx, c.httpClient, c.apiKey, c.model, body)
		if reqErr == nil {
			return result, nil
		}
		lastErr = reqErr
		if attempt < maxAITryCount {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(aiRetryDelay):
			}
		}
	}
	return "", fmt.Errorf("gemini request failed after retry: %w", lastErr)
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	Temperature      float32 `json:"temperature,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func doGeminiRequest(ctx context.Context, httpClient *http.Client, apiKey string, model string, body []byte) (string, error) {
	requestURL, err := buildGeminiURL(model, apiKey)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("gemini status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var response geminiResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("gemini decode failed: %w", err)
	}
	if len(response.Candidates) == 0 {
		return "", fmt.Errorf("gemini response had no candidates")
	}
	var builder strings.Builder
	for _, part := range response.Candidates[0].Content.Parts {
		builder.WriteString(part.Text)
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", fmt.Errorf("gemini response had empty text")
	}
	return text, nil
}

func buildGeminiURL(model string, apiKey string) (string, error) {
	if model == "" {
		return "", fmt.Errorf("gemini model is not configured")
	}
	if apiKey == "" {
		return "", fmt.Errorf("gemini api key is not configured")
	}
	escapedModel := url.PathEscape(model)
	endpoint := fmt.Sprintf("%s%s:generateContent", geminiBaseURL, escapedModel)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("key", apiKey)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
