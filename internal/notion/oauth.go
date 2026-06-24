package notion

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	authorizationURL = "https://api.notion.com/v1/oauth/authorize"
	tokenURL         = "https://api.notion.com/v1/oauth/token"
)

// OAuthConfig holds the registered Notion public integration's credentials.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// BuildAuthorizationURL returns the URL to redirect the user to so they can
// grant ChatVault's Notion integration access to a workspace. state must be
// an unguessable, per-request value the caller verifies on callback (CSRF
// protection). owner=user is required by Notion so the grant flow lets the
// user pick which pages/databases to share, rather than the whole workspace.
func BuildAuthorizationURL(cfg OAuthConfig, state string) string {
	oauth2Cfg := &oauth2.Config{
		ClientID:    cfg.ClientID,
		RedirectURL: cfg.RedirectURL,
		Endpoint:    oauth2.Endpoint{AuthURL: authorizationURL},
	}
	return oauth2Cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("owner", "user"))
}

// OAuthToken holds the access token returned by Notion plus the workspace it
// is scoped to (Notion's OAuth grant is per-workspace, not per-database).
type OAuthToken struct {
	AccessToken   string
	WorkspaceID   string
	WorkspaceName string
}

// ExchangeCodeForToken exchanges an authorization code for an access token.
// Notion's token endpoint requires a JSON request body and HTTP Basic auth,
// which differs from golang.org/x/oauth2's default form-encoded exchange, so
// this is implemented as a direct HTTP call rather than via oauth2.Config.Exchange.
func ExchangeCodeForToken(ctx context.Context, httpClient *http.Client, cfg OAuthConfig, code string) (OAuthToken, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type":   "authorization_code",
		"code":         code,
		"redirect_uri": cfg.RedirectURL,
	})
	if err != nil {
		return OAuthToken{}, fmt.Errorf("marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	credentials := base64.StdEncoding.EncodeToString([]byte(cfg.ClientID + ":" + cfg.ClientSecret))
	req.Header.Set("Authorization", "Basic "+credentials)

	resp, err := httpClient.Do(req)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("exchange notion oauth code: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken   string `json:"access_token"`
		WorkspaceID   string `json:"workspace_id"`
		WorkspaceName string `json:"workspace_name"`
		Error         string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return OAuthToken{}, fmt.Errorf("decode token response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest || result.Error != "" {
		return OAuthToken{}, fmt.Errorf("notion oauth token exchange failed: status=%d error=%s", resp.StatusCode, result.Error)
	}

	return OAuthToken{
		AccessToken:   result.AccessToken,
		WorkspaceID:   result.WorkspaceID,
		WorkspaceName: result.WorkspaceName,
	}, nil
}
