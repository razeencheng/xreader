package ai

import "context"

type AIClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type MockClient struct {
	Response ChatResponse
	Err      error
	Calls    []ChatRequest
}

func (m *MockClient) ChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	m.Calls = append(m.Calls, req)
	return m.Response, m.Err
}

var _ AIClient = (*Client)(nil)
var _ AIClient = (*MockClient)(nil)
