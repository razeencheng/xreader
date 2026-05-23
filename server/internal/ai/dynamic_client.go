package ai

import (
	"context"
	"errors"
	"sync"
)

type DynamicClient struct {
	settings   *SettingsService
	mu         sync.Mutex
	cachedCfg  ResolvedConfig
	cachedCli  *Client
}

func NewDynamicClient(settings *SettingsService) *DynamicClient {
	return &DynamicClient{settings: settings}
}

func (c *DynamicClient) ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if c == nil || c.settings == nil {
		return ChatResponse{}, errors.New("AI settings not configured")
	}
	cfg, err := c.settings.LoadResolved(ctx)
	if err != nil {
		return ChatResponse{}, err
	}
	if cfg.APIKey == "" {
		return ChatResponse{}, errors.New("AI API key not configured")
	}

	client := c.getOrCreateClient(cfg)
	return client.ChatCompletion(ctx, req)
}

func (c *DynamicClient) getOrCreateClient(cfg ResolvedConfig) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedCli != nil &&
		c.cachedCfg.BaseURL == cfg.BaseURL &&
		c.cachedCfg.APIKey == cfg.APIKey &&
		c.cachedCfg.Model == cfg.Model &&
		c.cachedCfg.MaxRetries == cfg.MaxRetries &&
		c.cachedCfg.Timeout == cfg.Timeout {
		return c.cachedCli
	}

	c.cachedCfg = cfg
	c.cachedCli = NewClient(cfg)
	return c.cachedCli
}
