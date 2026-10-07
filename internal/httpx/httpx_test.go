package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"name": "Tom & <Jerry>"})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if got, want := rec.Body.String(), "{\"name\":\"Tom \\u0026 \\u003cJerry\\u003e\"}\n"; got != want {
		t.Errorf("body %q, want %q", got, want)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusNotFound, "hero_not_found", "Hero was not found in the dataset.")
	want := `{"error":{"code":"hero_not_found","message":"Hero was not found in the dataset."}}` + "\n"
	if rec.Code != http.StatusNotFound || rec.Body.String() != want {
		t.Errorf("got %d %q, want 404 %q", rec.Code, rec.Body.String(), want)
	}
}

func TestInternalErrorHidesTheCause(t *testing.T) {
	rec := httptest.NewRecorder()
	InternalError(rec, "list heroes", errors.New("password authentication failed for user postgres"))
	want := `{"error":{"code":"internal_error","message":"Internal server error."}}` + "\n"
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != want {
		t.Errorf("got %d %q, want 500 %q", rec.Code, rec.Body.String(), want)
	}
}

func TestRouterAnswersUnmatchedRequestsInJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/heroes", func(w http.ResponseWriter, _ *http.Request) { WriteJSON(w, http.StatusOK, "heroes") })
	h := Router(mux)

	ok := serve(h, http.MethodGet, "/api/heroes", nil)
	if ok.Code != http.StatusOK || ok.Body.String() != `"heroes"`+"\n" {
		t.Errorf("matched route: got %d %q", ok.Code, ok.Body.String())
	}

	notFound := serve(h, http.MethodGet, "/nope", nil)
	if want := `{"error":{"code":"not_found","message":"Route was not found."}}` + "\n"; notFound.Code != http.StatusNotFound || notFound.Body.String() != want {
		t.Errorf("unknown path: got %d %q, want 404 %q", notFound.Code, notFound.Body.String(), want)
	}

	wrongMethod := serve(h, http.MethodDelete, "/api/heroes", nil)
	if want := `{"error":{"code":"method_not_allowed","message":"Method is not allowed on this route."}}` + "\n"; wrongMethod.Code != http.StatusMethodNotAllowed || wrongMethod.Body.String() != want {
		t.Errorf("wrong method: got %d %q, want 405 %q", wrongMethod.Code, wrongMethod.Body.String(), want)
	}
	if allow := wrongMethod.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow = %q, want GET, HEAD", allow)
	}

	unclean := serve(h, http.MethodGet, "/api//heroes", nil)
	if unclean.Code != http.StatusTemporaryRedirect || unclean.Header().Get("Location") != "/api/heroes" {
		t.Errorf("unclean path: got %d to %q, want the mux's redirect to /api/heroes", unclean.Code, unclean.Header().Get("Location"))
	}
}

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { WriteJSON(w, http.StatusOK, "ok") })
	h := CORS([]string{"https://front.example"}, next)

	for _, origin := range []string{"http://localhost:3000", "https://front.example"} {
		rec := serve(h, http.MethodGet, "/health", map[string]string{"Origin": origin})
		got := [3]string{rec.Header().Get("Access-Control-Allow-Origin"), rec.Header().Get("Access-Control-Allow-Credentials"), rec.Header().Get("Vary")}
		if want := [3]string{origin, "true", "Origin"}; got != want || rec.Code != http.StatusOK {
			t.Errorf("%s: got %d %q, want 200 %q", origin, rec.Code, got, want)
		}
	}

	other := serve(h, http.MethodGet, "/health", map[string]string{"Origin": "https://evil.example"})
	if v := other.Header().Get("Access-Control-Allow-Origin"); v != "" || other.Header().Get("Vary") != "" || other.Code != http.StatusOK {
		t.Errorf("other origin: got %d with Allow-Origin %q", other.Code, v)
	}

	preflight := serve(h, http.MethodOptions, "/api/heroes", map[string]string{
		"Origin": "http://localhost:3000", "Access-Control-Request-Headers": "content-type",
	})
	if preflight.Code != http.StatusNoContent ||
		preflight.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" ||
		preflight.Header().Get("Access-Control-Allow-Headers") != "content-type" ||
		preflight.Body.Len() != 0 {
		t.Errorf("preflight: got %d %v %q", preflight.Code, preflight.Header(), preflight.Body.String())
	}

	otherPreflight := serve(h, http.MethodOptions, "/api/heroes", map[string]string{"Origin": "https://evil.example"})
	if otherPreflight.Code != http.StatusOK || otherPreflight.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Errorf("preflight from another origin reached CORS handling: %d %v", otherPreflight.Code, otherPreflight.Header())
	}
}

func TestDocs(t *testing.T) {
	mux := http.NewServeMux()
	Docs(mux, []byte("openapi: 3.1.0\n"))
	h := Router(mux)

	// /docs/index.html is where browsers that once saw dev's permanent
	// redirect from /docs still go, so it serves the page rather than
	// redirecting back to /docs.
	for _, target := range []string{"/docs", "/docs/", "/docs/index.html"} {
		page := serve(h, http.MethodGet, target, nil)
		if page.Code != http.StatusOK || page.Header().Get("Content-Type") != "text/html; charset=utf-8" ||
			!strings.Contains(page.Body.String(), "url: '/docs/openapi.yaml'") {
			t.Errorf("%s: got %d %q, want the docs page", target, page.Code, page.Header().Get("Content-Type"))
		}
	}

	spec := serve(h, http.MethodGet, "/docs/openapi.yaml", nil)
	if spec.Code != http.StatusOK || spec.Header().Get("Content-Type") != "application/yaml" || spec.Body.String() != "openapi: 3.1.0\n" {
		t.Errorf("/docs/openapi.yaml: got %d %q %q", spec.Code, spec.Header().Get("Content-Type"), spec.Body.String())
	}

	root := serve(h, http.MethodGet, "/", nil)
	if root.Code != http.StatusFound || root.Header().Get("Location") != "/docs" {
		t.Errorf("/: got %d to %q, want 302 to /docs", root.Code, root.Header().Get("Location"))
	}
	if rec := serve(h, http.MethodGet, "/docs/other", nil); rec.Code != http.StatusNotFound {
		t.Errorf("/docs/other: got %d, want the JSON 404", rec.Code)
	}
}
