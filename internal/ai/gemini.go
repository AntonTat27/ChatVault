package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"chatvault/internal/model"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	maxAITryCount      = 2
	aiRetryDelay       = 5 * time.Second
	geminiTemperature  = 0.0
	geminiResponseJSON = "application/json"
	geminiResponseText = "text/plain"
	candidateCount     = 1
	topP               = 0.9
	topK               = 40
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
- "summary": 1-2 sentence narrative overview of the day's discussion (<= 30 words)
- "decisions": array of strings, each a clear decision (limit to top 3)
- "action_items": array of objects with "task" and "owner" (limit to top 3)
- "ideas": array of strings, each a notable idea (limit to top 3)
- "open_questions": array of strings, each an unresolved question (limit to top 3)

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
	// Use a transport wrapper that injects the x-goog-api-key header on every
	// HTTP request as a fallback for callers that require the API key header.
	// We still pass option.WithAPIKey to the SDK, but some environments require
	// the header too (or have restrictions). This makes requests more robust.
	transport := &apiKeyTransport{apiKey: apiKey, base: http.DefaultTransport}
	httpClient := &http.Client{Timeout: timeout, Transport: transport}

	return &GeminiClient{
		httpClient: httpClient,
		apiKey:     apiKey,
		model:      modelName,
	}
}

// apiKeyTransport injects x-goog-api-key header into outgoing requests.
type apiKeyTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.apiKey != "" {
		// clone request to avoid mutating shared objects
		r2 := req.Clone(req.Context())
		r2.Header.Set("x-goog-api-key", t.apiKey)
		return t.base.RoundTrip(r2)
	}
	return t.base.RoundTrip(req)
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
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"type":  {Type: genai.TypeString},
			"topic": {Type: genai.TypeString, Nullable: true},
		},
		Required: []string{"type"},
	}
	raw, err := c.sendPrompt(ctx, prompt, schema, 256)
	if err != nil {
		return model.ClassificationResult{}, err
	}
	return ParseClassificationResult(raw)
}

// GenerateSummary requests the daily summary JSON payload from Gemini.
func (c *GeminiClient) GenerateSummary(ctx context.Context, messagesJSON string) (model.DailySummary, error) {
	prompt := BuildSummaryPrompt(messagesJSON)
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"summary":        {Type: genai.TypeString},
			"decisions":      {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"action_items":   {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"task": {Type: genai.TypeString}, "owner": {Type: genai.TypeString, Nullable: true}}, Required: []string{"task"}}},
			"ideas":          {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"open_questions": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		},
		Required: []string{"summary"},
	}
	raw, err := c.sendPrompt(ctx, prompt, schema, 512)
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
	if idx := strings.LastIndex(candidate, "{"); idx > 0 {
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
func (c *GeminiClient) sendPrompt(ctx context.Context, prompt string, schema *genai.Schema, maxTokens int32) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("gemini api key is not configured")
	}
	if c.model == "" {
		return "", fmt.Errorf("gemini model is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= maxAITryCount; attempt++ {
		result, reqErr := c.doGenerate(ctx, prompt, schema, maxTokens)
		if reqErr == nil {
			return result, nil
		}
		lastErr = reqErr
		// If the model returned invalid JSON, retry once with a stricter, shorter prompt.
		if attempt == 1 && strings.Contains(reqErr.Error(), "invalid JSON") {
			prompt = buildStrictPrompt(prompt)
		}
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

func (c *GeminiClient) doGenerate(ctx context.Context, prompt string, schema *genai.Schema, maxTokens int32) (string, error) {
	if c.model == "" {
		return "", fmt.Errorf("gemini model is not configured")
	}
	// Create a fresh SDK client for every request so each prompt is fully isolated.
	// The Gemini API is stateless for GenerateContent, but this avoids any chance
	// of cross-request state through cached content or reused model settings.
	client, err := genai.NewClient(ctx, option.WithAPIKey(c.apiKey), option.WithHTTPClient(c.httpClient))
	if err != nil {
		return "", err
	}
	defer client.Close()

	generativeModel := client.GenerativeModel(c.model)
	// Use deterministic, low-cost settings to avoid repetition/degeneration and
	// keep token usage low. These defaults can be tuned later.
	generativeModel.SetMaxOutputTokens(maxTokens)
	generativeModel.SetTemperature(float32(geminiTemperature))
	// Sampling constraints
	generativeModel.SetCandidateCount(int32(candidateCount))
	generativeModel.SetTopP(float32(topP))
	generativeModel.SetTopK(int32(topK))
	// Enforce strict JSON output and a schema so the model returns only the
	// structured JSON we expect (no analysis/preamble). This also helps avoid
	// wasting tokens on intermediate "thinking" text.
	generativeModel.ResponseMIMEType = geminiResponseJSON
	if schema != nil {
		generativeModel.ResponseSchema = schema
	}

	resp, err := generativeModel.GenerateContent(ctx, genai.Text(prompt))

	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("gemini response had no candidates")
	}
	// If the API reports prompt feedback (blocked or similar), propagate a
	// clear error instead of attempting to parse.
	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != genai.BlockReasonUnspecified {
		return "", fmt.Errorf("prompt blocked: %v", resp.PromptFeedback.BlockReason)
	}
	if resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("gemini candidate contained no content")
	}

	var builder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		switch text := part.(type) {
		case genai.Text:
			builder.WriteString(string(text))
		default:
			builder.WriteString(fmt.Sprint(text))
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", fmt.Errorf("gemini response had empty text")
	}
	// Validate JSON strictly — if invalid, try to salvage the JSON object once.
	if !json.Valid([]byte(text)) {
		if candidate, ok := extractJSONCandidate(text); ok {
			return candidate, nil
		}
		return "", fmt.Errorf("invalid JSON returned by model: %q", text)
	}
	// Log usage metrics when available to help tune token limits.
	if resp.UsageMetadata != nil {
		log.Printf("ai usage prompt_tokens=%d candidates_tokens=%d total=%d", resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, resp.UsageMetadata.TotalTokenCount)
	}
	return text, nil
}

func buildStrictPrompt(base string) string {
	return base + "\n\nReturn only valid JSON that matches the response schema. " +
		"Do not add commentary or repeated text. If unsure, use empty arrays and a short summary (<= 40 words)."
}

func extractJSONCandidate(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < 0 || end <= start {
		return "", false
	}
	candidate := strings.TrimSpace(raw[start : end+1])
	if json.Valid([]byte(candidate)) {
		return candidate, true
	}
	return "", false
}
