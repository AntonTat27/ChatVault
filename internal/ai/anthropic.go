package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chatvault/internal/model"
)

const (
	anthropicURL           = "https://api.anthropic.com/v1/messages"
	maxAITryCount          = 2
	aiRetryDelay           = 5 * time.Second
	anthropicVersionHeader = "2023-06-01"
	anthropicMaxTokens     = 512
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

// AnthropicClient performs AI classification and summary requests.
type AnthropicClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

// NewAnthropicClient constructs AnthropicClient with request timeout.
func NewAnthropicClient(apiKey string, modelName string, timeout time.Duration) *AnthropicClient {
	return &AnthropicClient{
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

// ClassifyMessage tags a message using Anthropic and parses strict JSON output.
func (c *AnthropicClient) ClassifyMessage(ctx context.Context, messageText string) (model.ClassificationResult, error) {
	prompt := BuildClassificationPrompt(messageText)
	raw, err := c.sendPrompt(ctx, prompt)
	if err != nil {
		return model.ClassificationResult{}, err
	}
	return ParseClassificationResult(raw)
}

// GenerateSummary requests the daily summary JSON payload from Anthropic.
func (c *AnthropicClient) GenerateSummary(ctx context.Context, messagesJSON string) (model.DailySummary, error) {
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

// sendPrompt executes an Anthropic request with retry/backoff policy.
func (c *AnthropicClient) sendPrompt(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("anthropic api key is not configured")
	}

	requestBody := map[string]any{
		"model":      c.model,
		"max_tokens": anthropicMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 1; attempt <= maxAITryCount; attempt++ {
		result, reqErr := c.doRequest(ctx, body)
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
	return "", fmt.Errorf("anthropic request failed after retry: %w", lastErr)
}

// doRequest executes one HTTP request to Anthropic.
func (c *AnthropicClient) doRequest(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersionHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("anthropic status %d", resp.StatusCode)
	}

	type contentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type anthropicResponse struct {
		Content []contentBlock `json:"content"`
	}

	var response anthropicResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("anthropic decode failed: %w", err)
	}
	if len(response.Content) == 0 {
		return "", fmt.Errorf("anthropic response had no content")
	}
	return strings.TrimSpace(response.Content[0].Text), nil
}
