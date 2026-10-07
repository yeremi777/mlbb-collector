// Package ai asks a chat model for a JSON object: one OpenAI-compatible
// provider, or a chain of them that falls through on a retryable failure.
package ai

// Message is one turn of a chat exchange.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
