package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"chatvault/internal/model"
)

const (
	createPageURL = "https://api.notion.com/v1/pages"
	notionAPIVersion = "2022-06-28"
)

// Client pushes daily summaries into Notion databases.
type Client struct {
	httpClient *http.Client
	version    string
}

// NewClient constructs a Notion API client.
func NewClient(timeout time.Duration, version string) *Client {
	if version == "" {
		version = notionAPIVersion
	}
	return &Client{httpClient: &http.Client{Timeout: timeout}, version: version}
}

// CreateDailySummaryPage creates a Notion page from a daily summary with rich metadata.
func (c *Client) CreateDailySummaryPage(ctx context.Context, cfg model.NotionConfig, summary model.DailySummary, chatName string) error {
	if !cfg.Configured {
		return nil
	}

	payload := map[string]any{
		"parent":     map[string]string{"database_id": cfg.DatabaseID},
		"properties": buildSummaryProperties(summary, chatName),
		"children":  buildSummaryBlocks(summary),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createPageURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Notion-Version", c.version)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		errMsg := bytes.TrimSpace(body)
		if len(errMsg) > 0 {
			return fmt.Errorf("notion API error %d: %s", resp.StatusCode, string(errMsg))
		}
		return fmt.Errorf("notion API error %d", resp.StatusCode)
	}
	return nil
}

// buildSummaryProperties constructs database properties for a daily summary page.
func buildSummaryProperties(summary model.DailySummary, chatName string) map[string]any {
	props := map[string]any{
		"Name": map[string]any{
			"title": []map[string]any{
				{
					"type": "text",
					"text": map[string]string{
						"content": fmt.Sprintf("Daily Summary — %s", summary.SummaryDateUTC),
					},
				},
			},
		},
		"Date": map[string]any{
			"date": map[string]string{
				"start": summary.SummaryDateUTC,
			},
		},
		"Chat": map[string]any{
			"rich_text": []map[string]any{
				{
					"type": "text",
					"text": map[string]string{
						"content": chatName,
					},
				},
			},
		},
		"Message Count": map[string]any{
			"number": summary.MessageCount,
		},
	}

	// Add optional property for number of action items if supported
	if len(summary.ActionItems) > 0 {
		props["Action Items"] = map[string]any{
			"number": len(summary.ActionItems),
		}
	}

	// Add optional property for number of decisions if supported
	if len(summary.Decisions) > 0 {
		props["Decisions"] = map[string]any{
			"number": len(summary.Decisions),
		}
	}

	return props
}

// buildSummaryBlocks transforms summary sections into Notion block objects with rich formatting.
func buildSummaryBlocks(summary model.DailySummary) []map[string]any {
	blocks := []map[string]any{}

	// Overview section
	blocks = append(blocks, headingBlock("Summary", true))
	if summary.Summary != "" {
		blocks = append(blocks, paragraphBlock(summary.Summary))
	} else {
		blocks = append(blocks, paragraphBlock("No summary available."))
	}

	// Decisions section
	blocks = append(blocks, headingBlock("Decisions", true))
	if len(summary.Decisions) > 0 {
		for _, decision := range summary.Decisions {
			blocks = append(blocks, bulletBlock(decision, ""))
		}
	} else {
		blocks = append(blocks, bulletBlock("No decisions recorded.", "gray"))
	}

	// Action Items section
	blocks = append(blocks, headingBlock("Action Items", true))
	if len(summary.ActionItems) > 0 {
		for _, action := range summary.ActionItems {
			owner := "unassigned"
			if action.Owner != nil && *action.Owner != "" {
				owner = *action.Owner
			}
			taskText := fmt.Sprintf("%s — %s", action.Task, owner)
			blocks = append(blocks, todoBlock(taskText))
		}
	} else {
		blocks = append(blocks, bulletBlock("No action items assigned.", "gray"))
	}

	// Ideas section
	blocks = append(blocks, headingBlock("Ideas", true))
	if len(summary.Ideas) > 0 {
		for _, idea := range summary.Ideas {
			blocks = append(blocks, bulletBlock(idea, ""))
		}
	} else {
		blocks = append(blocks, bulletBlock("No ideas shared.", "gray"))
	}

	// Open Questions section
	blocks = append(blocks, headingBlock("Open Questions", true))
	if len(summary.OpenQuestions) > 0 {
		for _, question := range summary.OpenQuestions {
			blocks = append(blocks, bulletBlock(question, ""))
		}
	} else {
		blocks = append(blocks, bulletBlock("No open questions.", "gray"))
	}

	return blocks
}

// headingBlock creates a heading_2 block with optional bold formatting.
func headingBlock(text string, bold bool) map[string]any {
	textObj := map[string]any{
		"type": "text",
		"text": map[string]string{
			"content": text,
		},
	}
	if bold {
		textObj["annotations"] = map[string]bool{
			"bold": true,
		}
	}

	return map[string]any{
		"object": "block",
		"type":   "heading_2",
		"heading_2": map[string]any{
			"rich_text": []map[string]any{textObj},
			"color":     "default",
		},
	}
}

// paragraphBlock creates a paragraph block for longer text content.
func paragraphBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]any{
			"rich_text": []map[string]any{
				{
					"type": "text",
					"text": map[string]string{
						"content": text,
					},
				},
			},
			"color": "default",
		},
	}
}

// bulletBlock creates a bulleted_list_item block with optional color support.
func bulletBlock(text string, color string) map[string]any {
	if color == "" {
		color = "default"
	}
	return map[string]any{
		"object": "block",
		"type":   "bulleted_list_item",
		"bulleted_list_item": map[string]any{
			"rich_text": []map[string]any{
				{
					"type": "text",
					"text": map[string]string{
						"content": text,
					},
				},
			},
			"color": color,
		},
	}
}

// todoBlock creates a to_do block with proper formatting.
func todoBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "to_do",
		"to_do": map[string]any{
			"checked": false,
			"rich_text": []map[string]any{
				{
					"type": "text",
					"text": map[string]string{
						"content": text,
					},
				},
			},
			"color": "default",
		},
	}
}
