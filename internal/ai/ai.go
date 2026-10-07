// Package ai asks a chat model for a JSON object: one OpenAI-compatible
// provider, or a chain of them that falls through on a retryable failure.
package ai

import "context"

// Message is one turn of a chat exchange.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider completes a chat exchange and returns the JSON object the model
// answered, with any markdown code fence around it removed.
type Provider interface {
	Name() string
	CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error)
}

// Error is a provider failure: the request failed, the provider answered an
// error status, or the model's answer is not a JSON object. Retryable lets a
// chain ask the next provider.
type Error struct {
	Message   string
	Retryable bool
}

func (e *Error) Error() string { return e.Message }
