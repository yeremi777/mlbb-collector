// Package httpx holds what every route shares: JSON bodies, the error
// envelope, JSON answers for unmatched requests, and CORS.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
)

// WriteJSON writes body as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// WriteError writes the error envelope {"error":{"code","message"}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code, body.Error.Message = code, message
	WriteJSON(w, status, body)
}

// InternalError logs err under op and answers 500 without revealing it.
func InternalError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
}

// Router serves mux, answering a request no route matches with a JSON 404,
// or a JSON 405 when the path exists under another method.
func Router(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		// No pattern matched: the mux would answer in plain text, or redirect
		// an unclean path. Learn which without letting it write.
		probe := &statusProbe{header: http.Header{}}
		h.ServeHTTP(probe, r)
		switch probe.status {
		case http.StatusNotFound:
			WriteError(w, http.StatusNotFound, "not_found", "Route was not found.")
		case http.StatusMethodNotAllowed:
			w.Header().Set("Allow", probe.header.Get("Allow"))
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed on this route.")
		default:
			h.ServeHTTP(w, r)
		}
	})
}

// statusProbe records the status and headers a handler writes, discarding
// its body.
type statusProbe struct {
	header http.Header
	status int
}

func (p *statusProbe) Header() http.Header         { return p.header }
func (p *statusProbe) Write(b []byte) (int, error) { return len(b), nil }
func (p *statusProbe) WriteHeader(status int)      { p.status = status }

// localFrontend is the frontend's development origin, always allowed.
const localFrontend = "http://localhost:3000"

// CORS lets the frontend origins call next with credentials, and answers
// their preflight requests itself. Any other origin gets no CORS headers.
func CORS(origins []string, next http.Handler) http.Handler {
	allowed := append([]string{localFrontend}, origins...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !slices.Contains(allowed, origin) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Mux is where routes register: an *http.ServeMux in the server.
type Mux interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// docsPage renders /docs/openapi.yaml with Scalar.
const docsPage = `<!doctype html>
<html>
<head>
<title>MLBB Collector API</title>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
<script id="api-reference" data-url="/docs/openapi.yaml"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1"></script>
</body>
</html>
`

// docsRedirects are addresses people reach for the docs by: the server's
// root, a trailing slash, and the Swagger UI address dev served.
var docsRedirects = []string{"GET /{$}", "GET /docs/{$}", "GET /docs/index.html"}

// Docs serves the OpenAPI contract at /docs/openapi.yaml and a Scalar page
// rendering it at /docs, and sends the docsRedirects addresses to /docs.
func Docs(mux Mux, spec []byte) {
	for _, pattern := range docsRedirects {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs", http.StatusFound)
		})
	}
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(docsPage))
	})
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
}
