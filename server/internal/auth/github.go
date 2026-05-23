package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	oauthgithub "golang.org/x/oauth2/github"
)

// OAuthConfig provides GitHub OAuth credentials. Resolved on each call
// so that Setup Wizard changes take effect without a server restart.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// OAuthConfigProvider returns the current OAuth configuration.
// The router wires this to ConfigResolver so it reads env-first, DB-fallback.
type OAuthConfigProvider func() OAuthConfig

type RealGitHubClient struct {
	configProvider OAuthConfigProvider
}

func NewGitHubClient(provider OAuthConfigProvider) *RealGitHubClient {
	return &RealGitHubClient{configProvider: provider}
}

func (c *RealGitHubClient) oauthConfig() *oauth2.Config {
	cfg := c.configProvider()
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.CallbackURL,
		Endpoint:     oauthgithub.Endpoint,
		Scopes:       []string{"read:user"},
	}
}

func (c *RealGitHubClient) AuthCodeURL(state string) string {
	return c.oauthConfig().AuthCodeURL(state)
}

func (c *RealGitHubClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	token, err := c.oauthConfig().Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	return token.AccessToken, nil
}

func (c *RealGitHubClient) FetchUser(ctx context.Context, token string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var ghResp struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghResp); err != nil {
		return nil, err
	}

	return &GitHubUser{
		GitHubID:  ghResp.ID,
		Username:  ghResp.Login,
		AvatarURL: ghResp.AvatarURL,
	}, nil
}
