package main

import (
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// recorder keeps the patterns registered on it.
type recorder struct{ patterns []string }

func (r *recorder) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
}

// documentedRoutes reads docs/openapi.yaml as "METHOD /path" patterns.
func documentedRoutes(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var routes []string
	for path, ops := range spec.Paths {
		for method := range ops {
			routes = append(routes, strings.ToUpper(method)+" "+path)
		}
	}
	return routes
}

func TestEveryRouteIsDocumentedAndEveryDocumentedRouteExists(t *testing.T) {
	rec := &recorder{}
	register(rec, nil, []byte("openapi: 3.1.0\n"))
	if len(rec.patterns) == 0 {
		t.Fatal("register registered nothing")
	}

	documented := documentedRoutes(t)
	servesDocs := []string{"GET /docs", "GET /docs/openapi.yaml"}
	analysisOwns := []string{
		"POST /api/counters/analyze-score", "POST /api/counters/analyze-detail",
		"POST /api/synergies/analyze-score", "POST /api/synergies/analyze-detail",
	}
	for _, p := range rec.patterns {
		if !slices.Contains(servesDocs, p) && !slices.Contains(documented, p) {
			t.Errorf("%s is registered but not in docs/openapi.yaml", p)
		}
	}
	for _, d := range documented {
		if !slices.Contains(rec.patterns, d) && !slices.Contains(analysisOwns, d) {
			t.Errorf("%s is in docs/openapi.yaml but not registered", d)
		}
	}
}
