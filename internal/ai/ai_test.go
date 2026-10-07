package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// stub serves one fixed reply to /chat/completions and counts its requests.
type stub struct {
	*httptest.Server
	requests int
}

func newStub(t *testing.T, status int, body string) *stub {
	t.Helper()
	s := &stub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests++
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func answer(content string) string {
	return `{"choices":[{"message":{"content":` + content + `}}]}`
}

func stubChat(t *testing.T, name, url string) Provider {
	t.Helper()
	p, err := NewChat(ChatConfig{Name: name, ServerURL: url + "/", APIKey: "key", Model: "model"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var question = []Message{{Role: "user", Content: "score"}}

func TestChatReturnsTheAnsweredObject(t *testing.T) {
	for name, content := range map[string]string{
		"plain":          `"{\"score\":90}"`,
		"fenced as json": `"` + "```json\\n{\\\"score\\\":90}\\n```" + `"`,
		"fenced":         `"` + "```\\n{\\\"score\\\":90}\\n```" + `"`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := stubChat(t, "One", newStub(t, 200, answer(content)).URL).CompleteJSON(context.Background(), question)
			if err != nil || !reflect.DeepEqual(got, map[string]any{"score": 90.0}) {
				t.Errorf("got %v, %v; want map[score:90]", got, err)
			}
		})
	}
}

func TestChatFailures(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	for _, tt := range []struct {
		name      string
		url       string
		message   string
		retryable bool
	}{
		{"empty message", newStub(t, 200, answer(`""`)).URL, "One returned an empty message.", false},
		{"not JSON", newStub(t, 200, answer(`"prose"`)).URL, "One returned invalid JSON.", false},
		{"body not JSON", newStub(t, 200, `<html>`).URL, "One returned invalid JSON.", false},
		{"error object", newStub(t, 500, `{"error":{"message":"upstream is down"}}`).URL, "One returned HTTP 500: upstream is down", true},
		{"error string", newStub(t, 503, `{"error":"overloaded"}`).URL, "One returned HTTP 503: overloaded", true},
		{"message", newStub(t, 429, `{"message":"slow down"}`).URL, "One returned HTTP 429: slow down", true},
		{"plain text", newStub(t, 401, `bad key`).URL, "One returned HTTP 401: bad key", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := stubChat(t, "One", tt.url).CompleteJSON(context.Background(), question)
			var pe *Error
			if !errors.As(err, &pe) || pe.Message != tt.message || pe.Retryable != tt.retryable {
				t.Errorf("got %#v, want %q retryable=%v", err, tt.message, tt.retryable)
			}
		})
	}

	_, err := stubChat(t, "One", closed.URL).CompleteJSON(context.Background(), question)
	var pe *Error
	if !errors.As(err, &pe) || !pe.Retryable {
		t.Errorf("unreachable provider: got %#v, want a retryable error", err)
	}
}

func TestNewChatRefusesAMissingKeyOrModel(t *testing.T) {
	for _, tt := range []struct {
		cfg  ChatConfig
		want string
	}{
		{ChatConfig{Name: "OpenRouter", Model: "m"}, "OpenRouter API key is not configured."},
		{ChatConfig{Name: "OpenRouter", APIKey: "k"}, "No model configured for OpenRouter."},
	} {
		if _, err := NewChat(tt.cfg); err == nil || err.Error() != tt.want {
			t.Errorf("got %v, want %q", err, tt.want)
		}
	}
}

func TestChainFallsThroughOnARetryableFailure(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	failing := map[string]string{"transport error": closed.URL}
	for _, status := range []int{402, 408, 409, 425, 429, 500, 502, 503} {
		failing[http.StatusText(status)] = newStub(t, status, `{"error":"no"}`).URL
	}
	for name, url := range failing {
		t.Run(name, func(t *testing.T) {
			next := newStub(t, 200, answer(`"{\"score\":90}"`))
			got, err := Chain(stubChat(t, "One", url), stubChat(t, "Two", next.URL)).CompleteJSON(context.Background(), question)
			if err != nil || !reflect.DeepEqual(got, map[string]any{"score": 90.0}) || next.requests != 1 {
				t.Errorf("got %v, %v after %d requests to the next provider; want its answer", got, err, next.requests)
			}
		})
	}
}

func TestChainStopsOnAFailureThatIsNotRetryable(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		next := newStub(t, 200, answer(`"{\"score\":90}"`))
		_, err := Chain(stubChat(t, "One", newStub(t, status, `{"error":"no"}`).URL), stubChat(t, "Two", next.URL)).
			CompleteJSON(context.Background(), question)
		want := fmt.Sprintf("One returned HTTP %d: no", status)
		if err == nil || err.Error() != want || next.requests != 0 {
			t.Errorf("%d: got %v after %d requests to the next provider; want One's error and none", status, err, next.requests)
		}
	}
}

func TestChainJoinsEveryFailureWhenAllFail(t *testing.T) {
	_, err := Chain(
		stubChat(t, "One", newStub(t, 429, `{"error":"rate limited"}`).URL),
		stubChat(t, "Two", newStub(t, 503, `{"error":"overloaded"}`).URL),
	).CompleteJSON(context.Background(), question)
	want := "All AI relay providers failed: One: One returned HTTP 429: rate limited | Two: Two returned HTTP 503: overloaded"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestChainOfOneIsThatProvider(t *testing.T) {
	_, err := Chain(stubChat(t, "One", newStub(t, 500, `{"error":"down"}`).URL)).CompleteJSON(context.Background(), question)
	if want := "One returned HTTP 500: down"; err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

// countingProvider fails retryably and counts its requests.
type countingProvider struct{ requests int }

func (p *countingProvider) Name() string { return "Counting" }

func (p *countingProvider) CompleteJSON(context.Context, []Message) (map[string]any, error) {
	p.requests++
	return nil, &Error{Message: "Counting failed.", Retryable: true}
}

func TestChainStopsOnceTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	first, next := &countingProvider{}, &countingProvider{}
	_, err := Chain(first, next).CompleteJSON(ctx, question)
	if err == nil || err.Error() != "Counting failed." || next.requests != 0 {
		t.Errorf("got %v after %d requests to the next provider; want the first failure and none", err, next.requests)
	}

	first, next = &countingProvider{}, &countingProvider{}
	if _, err := Chain(first, next).CompleteJSON(context.Background(), question); err == nil || next.requests != 1 {
		t.Errorf("live context: got %v after %d requests to the next provider; want 1", err, next.requests)
	}
}
