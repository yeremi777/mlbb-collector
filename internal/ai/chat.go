package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

// retryableStatuses fall through to the next provider, as does any 5xx.
var retryableStatuses = []int{402, 408, 409, 425, 429}

// ChatConfig is one OpenAI-compatible chat-completions provider. Headers are
// sent with every request.
type ChatConfig struct {
	Name      string
	ServerURL string
	APIKey    string
	Model     string
	Headers   map[string]string
}

type chat struct {
	cfg    ChatConfig
	client *http.Client
}

// NewChat asks the provider cfg describes, and refuses one without an API key
// or model. The caller's context bounds each request.
func NewChat(cfg ChatConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key is not configured.", cfg.Name)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("No model configured for %s.", cfg.Name)
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	return &chat{cfg: cfg, client: &http.Client{}}, nil
}

func (p *chat) Name() string { return p.cfg.Name }

func (p *chat) CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error) {
	body, err := json.Marshal(map[string]any{
		"model":           p.cfg.Model,
		"messages":        messages,
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.2,
	})
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ServerURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("%s request failed: %v", p.cfg.Name, err)
		if errors.Is(err, context.DeadlineExceeded) {
			msg = fmt.Sprintf("%s request timed out.", p.cfg.Name)
		}
		return nil, &Error{Message: msg, Retryable: true}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("%s response read failed: %v", p.cfg.Name, err), Retryable: true}
	}
	if resp.StatusCode >= 400 {
		return nil, &Error{
			Message:   fmt.Sprintf("%s returned HTTP %d: %s", p.cfg.Name, resp.StatusCode, errorDetail(raw)),
			Retryable: resp.StatusCode >= 500 || slices.Contains(retryableStatuses, resp.StatusCode),
		}
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &completion); err != nil {
		return nil, &Error{Message: fmt.Sprintf("%s returned invalid JSON.", p.cfg.Name)}
	}
	content := ""
	if len(completion.Choices) > 0 {
		content = completion.Choices[0].Message.Content
	}
	if content == "" {
		return nil, &Error{Message: fmt.Sprintf("%s returned an empty message.", p.cfg.Name)}
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(unfence(content)), &object); err != nil {
		return nil, &Error{Message: fmt.Sprintf("%s returned invalid JSON.", p.cfg.Name)}
	}
	return object, nil
}

// unfence removes a markdown code fence around the model's JSON.
func unfence(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

// errorDetail is the provider's error message from an error body, or the body
// itself cut to 500 bytes.
func errorDetail(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return truncate(string(raw))
	}
	if errObj, ok := payload["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if errStr, ok := payload["error"].(string); ok {
		return errStr
	}
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}
	return truncate(fmt.Sprintf("%v", payload))
}

func truncate(s string) string {
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
