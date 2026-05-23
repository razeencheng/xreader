package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_SendsCorrectOpenAIShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-xxx", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "my-model", body["model"])

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "hi"}},
			},
			"model": "my-model",
			"usage": map[string]any{"total_tokens": 10},
		})
	}))
	defer srv.Close()

	c := NewClient(ResolvedConfig{BaseURL: srv.URL, APIKey: "sk-xxx", Model: "default-model", MaxRetries: 0, Timeout: 5 * time.Second})
	resp, err := c.ChatCompletion(context.Background(), ChatRequest{
		Model:    "my-model",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	require.NoError(t, err)
	require.Equal(t, "hi", resp.Content)
}

func TestClient_RetriesOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
			"model": "m",
			"usage": map[string]any{"total_tokens": 5},
		})
	}))
	defer srv.Close()

	c := NewClient(ResolvedConfig{BaseURL: srv.URL, APIKey: "key", Model: "m", MaxRetries: 2, Timeout: 5 * time.Second})
	resp, err := c.ChatCompletion(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, int32(2), calls.Load())
}

func TestClient_FailsAfterMaxRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	c := NewClient(ResolvedConfig{BaseURL: srv.URL, APIKey: "key", Model: "m", MaxRetries: 1, Timeout: 5 * time.Second})
	_, err := c.ChatCompletion(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "retries exhausted")
}
