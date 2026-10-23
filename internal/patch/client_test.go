package patch_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/patch"
)

// page is a MediaWiki parse response carrying html.
func page(html string) string {
	b, _ := json.Marshal(map[string]any{"parse": map[string]any{"title": "Portal:Patches", "text": map[string]string{"*": html}}})
	return string(b)
}

var onePatch = page(table(`<tr><td>Patch 2.1.90</td><td>July 1, 2026</td><td><ul><li>Hero Adjustments</li></ul></td></tr>`))

type reply struct {
	status     int
	retryAfter string
	body       string
}

// stub answers each request with the next reply and records what it received.
type stub struct {
	mu       sync.Mutex
	replies  []reply
	requests []*http.Request
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r)
	next := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	if next.retryAfter != "" {
		w.Header().Set("Retry-After", next.retryAfter)
	}
	if next.status != 0 {
		w.WriteHeader(next.status)
	}
	_, _ = io.WriteString(w, next.body)
}

func newClient(t *testing.T, s *stub) (*patch.Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c := patch.NewClient(srv.URL+"/mobilelegends/api.php", "probe/1.0 (me@example.com)")
	var waits []time.Duration
	c.Sleep = func(d time.Duration) { waits = append(waits, d) }
	return c, &waits
}

func TestFetchMakesOneParseRequest(t *testing.T) {
	s := &stub{replies: []reply{{body: onePatch}}}
	c, waits := newClient(t, s)

	got, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != "2.1.90" || len(got[0].Highlights) != 1 {
		t.Errorf("got %+v", got)
	}
	if len(s.requests) != 1 || len(*waits) != 0 {
		t.Fatalf("%d requests, waits %v; want one request", len(s.requests), *waits)
	}
	r := s.requests[0]
	q := r.URL.Query()
	if r.Method != http.MethodGet || r.URL.Path != "/mobilelegends/api.php" ||
		q.Get("action") != "parse" || q.Get("page") != "Portal:Patches" || q.Get("format") != "json" || q.Get("prop") != "text" || len(q) != 4 {
		t.Errorf("request = %s %s", r.Method, r.URL)
	}
	if ua := r.Header.Get("User-Agent"); ua != "probe/1.0 (me@example.com)" {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestFetchRetriesOnce(t *testing.T) {
	for name, tc := range map[string]struct {
		first reply
		wait  time.Duration
	}{
		"429":                     {reply{status: 429}, 30 * time.Second},
		"429 with a longer wait":  {reply{status: 429, retryAfter: "45"}, 45 * time.Second},
		"503 with a shorter wait": {reply{status: 503, retryAfter: "5"}, 30 * time.Second},
		"body with no tables":     {reply{body: page("<p>Rate limited</p>")}, 30 * time.Second},
		"body that is not json":   {reply{body: "<html>busy</html>"}, 30 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{replies: []reply{tc.first, {body: onePatch}}}
			c, waits := newClient(t, s)
			got, err := c.Fetch(context.Background())
			if err != nil || len(got) != 1 {
				t.Fatalf("got %+v, err %v; want the retry's patch", got, err)
			}
			if len(s.requests) != 2 || len(*waits) != 1 || (*waits)[0] != tc.wait {
				t.Errorf("%d requests, waits %v; want 2 and one %s wait", len(s.requests), *waits, tc.wait)
			}
		})
	}
}

func TestFetchFailsAtOnce(t *testing.T) {
	for name, first := range map[string]reply{
		"mediawiki error":           {body: `{"error": {"code": "missingtitle", "info": "The page you specified doesn't exist."}}`},
		"status 404":                {status: 404},
		"tables without patch rows": {body: page(table(`<tr><td>Season 38</td><td>July 3, 2026</td></tr>`))},
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{replies: []reply{first}}
			c, waits := newClient(t, s)
			if _, err := c.Fetch(context.Background()); err == nil {
				t.Fatal("want an error")
			}
			if len(s.requests) != 1 || len(*waits) != 0 {
				t.Errorf("%d requests, waits %v; want one request", len(s.requests), *waits)
			}
		})
	}
}

func TestFetchGivesUpAfterTheRetry(t *testing.T) {
	s := &stub{replies: []reply{{status: 502}}}
	c, _ := newClient(t, s)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("want an error")
	}
	if len(s.requests) != 2 {
		t.Errorf("%d requests, want 2", len(s.requests))
	}
}

func TestFetchRetriesATransportError(t *testing.T) {
	c := patch.NewClient("http://127.0.0.1:1", "probe/1.0")
	var waits []time.Duration
	c.Sleep = func(d time.Duration) { waits = append(waits, d) }
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("want an error")
	}
	if len(waits) != 1 {
		t.Errorf("waits %v, want one retry", waits)
	}
}

func TestFetchDoesNotRetryACanceledRequest(t *testing.T) {
	s := &stub{replies: []reply{{body: onePatch}}}
	c, waits := newClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx); err == nil {
		t.Fatal("want an error")
	}
	if len(*waits) != 0 {
		t.Errorf("waits %v, want no retry", *waits)
	}
}
