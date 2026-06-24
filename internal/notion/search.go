package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const searchURL = "https://api.notion.com/v1/search"

// Database is a minimal reference to a Notion database, used by the
// dashboard's post-OAuth database picker (Notion's OAuth grant is
// workspace-scoped, so the user must pick a specific database afterward).
type Database struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SearchDatabases lists the databases an OAuth access token can see, using
// Notion's /v1/search endpoint filtered to objects of type "database".
func SearchDatabases(ctx context.Context, httpClient *http.Client, accessToken string, notionVersion string) ([]Database, error) {
	body, err := json.Marshal(map[string]any{
		"filter": map[string]string{"value": "database", "property": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Notion-Version", notionVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("notion search api error %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			ID    string `json:"id"`
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	databases := make([]Database, 0, len(result.Results))
	for _, r := range result.Results {
		title := ""
		for _, t := range r.Title {
			title += t.PlainText
		}
		if title == "" {
			title = "Untitled database"
		}
		databases = append(databases, Database{ID: r.ID, Title: title})
	}
	return databases, nil
}
