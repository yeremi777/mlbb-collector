package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Chain asks providers in order, moving to the next on a retryable failure. A
// chain of one is that provider, so its failures keep their own message.
func Chain(providers ...Provider) Provider {
	if len(providers) == 1 {
		return providers[0]
	}
	return chain(providers)
}

type chain []Provider

func (c chain) Name() string { return "Relay" }

func (c chain) CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error) {
	var failures []string
	for _, p := range c {
		object, err := p.CompleteJSON(ctx, messages)
		if err == nil {
			return object, nil
		}
		var pe *Error
		if !errors.As(err, &pe) || !pe.Retryable || ctx.Err() != nil {
			return nil, err
		}
		failures = append(failures, fmt.Sprintf("%s: %v", p.Name(), err))
		slog.Warn("ai provider failed, trying next", "provider", p.Name(), "err", err)
	}
	return nil, &Error{Message: "All AI relay providers failed: " + strings.Join(failures, " | ")}
}
