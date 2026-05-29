package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chatvault/internal/model"
)

const createPageURL = "https://api.notion.com/v1/pages"

// Client pushes daily summaries into Notion databases.
type Client struct {
	httpClient *http.Client
	version    string
}

// NewClient constructs a Notion API client.
func NewClient(timeout time.Duration, version string) *Client {
	return &Client{httpClient: &http.Client{Timeout: timeout}, version: version}
}

// CreateDailySummaryPage creates a Notion page from a daily summary.
func (c *Client) CreateDailySummaryPage(ctx context.Context, cfg model.NotionConfig, summary model.DailySummary, chatName string) error {
	if !cfg.Configured {
		return nil
	}

	payload := map[string]any{
		"parent": map[string]string{"database_id": cfg.DatabaseID},
		"properties": map[string]any{
			"Name": map[string]any{
				"title": []map[string]any{{"text": map[string]string{"content": fmt.Sprintf("[%s] — Daily Summary", summary.SummaryDateUTC)}}},
			},
			"Date":          map[string]any{"date": map[string]string{"start": summary.SummaryDateUTC}},
			"Chat Name":     map[string]any{"rich_text": []map[string]any{{"text": map[string]string{"content": chatName}}}},
			"Message Count": map[string]any{"number": summary.MessageCount},
		},
		"children": buildSummaryBlocks(summary),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createPageURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Notion-Version", c.version)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("notion status %d", resp.StatusCode)
	}
	return nil
}

// buildSummaryBlocks transforms summary sections into Notion block objects.
func buildSummaryBlocks(summary model.DailySummary) []map[string]any {
	blocks := []map[string]any{
		headingBlock("Summary"),
		bulletBlock(summary.Summary),
		headingBlock("Decisions"),
	}
	for _, decision := range summary.Decisions {
		blocks = append(blocks, bulletBlock(decision))
	}
	blocks = append(blocks, headingBlock("Action Items"))
	for _, action := range summary.ActionItems {
		owner := "unassigned"
		if action.Owner != nil && *action.Owner != "" {
			owner = *action.Owner
		}
		blocks = append(blocks, todoBlock(fmt.Sprintf("%s (%s)", action.Task, owner)))
	}
	blocks = append(blocks, headingBlock("Ideas"))
	for _, idea := range summary.Ideas {
		blocks = append(blocks, bulletBlock(idea))
	}
	blocks = append(blocks, headingBlock("Open Questions"))
	for _, question := range summary.OpenQuestions {
		blocks = append(blocks, bulletBlock(question))
	}
	return blocks
}

// headingBlock creates a heading_2 block.
func headingBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "heading_2",
		"heading_2": map[string]any{
			"rich_text": []map[string]any{{"type": "text", "text": map[string]string{"content": text}}},
		},
	}
}

// bulletBlock creates a bulleted_list_item block.
func bulletBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "bulleted_list_item",
		"bulleted_list_item": map[string]any{
			"rich_text": []map[string]any{{"type": "text", "text": map[string]string{"content": text}}},
		},
	}
}

// todoBlock creates a to_do block.
func todoBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "to_do",
		"to_do": map[string]any{
			"checked":   false,
			"rich_text": []map[string]any{{"type": "text", "text": map[string]string{"content": text}}},
		},
	}
}
