package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Content string
	Model   string
	Usage   Usage
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage Usage  `json:"usage"`
}

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	maxRetries int
	timeout    time.Duration
	httpClient *http.Client
}

func NewClient(cfg ResolvedConfig) *Client {
	return &Client{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxRetries: cfg.MaxRetries,
		timeout:    cfg.Timeout,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ChatResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
		}

		start := time.Now()
		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return ChatResponse{}, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			log.Printf("ai: attempt %d failed: %v", attempt+1, err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
			log.Printf("ai: attempt %d got %d, retrying", attempt+1, resp.StatusCode)
			continue
		}

		if resp.StatusCode != 200 {
			return ChatResponse{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		var oaiResp openAIResponse
		if err := json.Unmarshal(respBody, &oaiResp); err != nil {
			return ChatResponse{}, fmt.Errorf("parse response: %w", err)
		}

		latency := time.Since(start)
		log.Printf("ai: model=%s latency=%s tokens=%d", oaiResp.Model, latency, oaiResp.Usage.TotalTokens)

		if len(oaiResp.Choices) == 0 {
			return ChatResponse{}, fmt.Errorf("empty choices in response")
		}

		return ChatResponse{
			Content: oaiResp.Choices[0].Message.Content,
			Model:   oaiResp.Model,
			Usage:   oaiResp.Usage,
		}, nil
	}

	return ChatResponse{}, fmt.Errorf("all %d retries exhausted: %w", c.maxRetries+1, lastErr)
}
