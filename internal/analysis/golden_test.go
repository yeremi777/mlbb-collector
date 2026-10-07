package analysis

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// routeCase is one file of testdata/routes, captured from dev's analyzer and
// response encoding at b04727f: a request, the answers of a stub provider for
// each provider asked, and the status, body, and provider requests dev gave.
type routeCase struct {
	Route     string          `json:"route"`
	Request   json.RawMessage `json:"request"`
	Providers []stubProvider  `json:"providers"`
	Status    int             `json:"status"`
	Body      string          `json:"body"`
	Received  []received      `json:"received"`
}

type stubProvider struct {
	Provider string `json:"provider"`
	Answers  []struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	} `json:"answers"`
}

type received struct {
	Provider string            `json:"provider"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
}

var recordedHeaders = []string{"Authorization", "Content-Type", "HTTP-Referer", "X-OpenRouter-Title"}

// stubEnv starts a chat-completions stub per provider of c and returns the
// environment that points the providers at them, and the requests the stubs
// receive.
func stubEnv(t *testing.T, c routeCase) (map[string]string, *[]received) {
	t.Helper()
	var (
		mu  sync.Mutex
		log []received
	)
	env := map[string]string{"AI_ANALYSIS_CACHE_TTL_SECONDS": "0"}
	var names []string
	for _, sp := range c.Providers {
		next := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			headers := map[string]string{}
			for _, h := range recordedHeaders {
				if v := r.Header.Get(h); v != "" {
					headers[h] = v
				}
			}
			mu.Lock()
			defer mu.Unlock()
			log = append(log, received{Provider: sp.Provider, Path: r.URL.Path, Headers: headers, Body: string(body)})
			if next == len(sp.Answers) {
				t.Errorf("%s asked more often than dev asked it", sp.Provider)
				w.WriteHeader(http.StatusTeapot)
				return
			}
			a := sp.Answers[next]
			next++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(a.Status)
			_, _ = io.WriteString(w, a.Body)
		}))
		t.Cleanup(srv.Close)
		prefix := strings.ToUpper(sp.Provider)
		env[prefix+"_SERVER_URL"], env[prefix+"_API_KEY"], env[prefix+"_MODEL"] = srv.URL, "test-key", "test/model"
		names = append(names, sp.Provider)
	}
	env["AI_PROVIDERS"] = strings.Join(names, ",")
	return env, &log
}

func TestRoutesAnswerAsDevDid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "routes", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no route cases: %v", err)
	}
	stores := newDatasetStores(t)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var c routeCase
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			env, log := stubEnv(t, c)
			rec := stores.post(t, NewFromConfig(mustLoadAI(t, env)), c.Route, string(c.Request))
			if rec.Code != c.Status || rec.Body.String() != c.Body {
				t.Errorf("got %d %s\nwant %d %s", rec.Code, rec.Body, c.Status, c.Body)
			}
			if !reflect.DeepEqual(*log, c.Received) {
				t.Errorf("providers received\n%+v\nwant\n%+v", *log, c.Received)
			}
		})
	}
}
