package ai

import "time"

type ResolvedConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxRetries int
	Timeout    time.Duration
	BatchSize  int
}
