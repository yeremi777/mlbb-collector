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
