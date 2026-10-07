package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/ai"
)

func TestLoadAIDefaults(t *testing.T) {
	got, err := LoadAI(env(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	want := AI{
		Timeout:         60 * time.Second,
		CacheTTL:        600 * time.Second,
		CacheMaxEntries: 256,
		OpenRouter: ai.ChatConfig{Name: "OpenRouter", ServerURL: "https://openrouter.ai/api/v1", Model: "openrouter/free",
			Headers: map[string]string{"HTTP-Referer": "http://127.0.0.1:8000", "X-OpenRouter-Title": "MLBB Analyzer Service"}},
		OpenCodeZen: ai.ChatConfig{Name: "OpenCode Zen", ServerURL: "https://opencode.ai/zen/v1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestLoadAI(t *testing.T) {
	got, err := LoadAI(env(map[string]string{
		"AI_PROVIDERS":                  " OpenRouter, ,opencode_zen,mock",
		"AI_TIMEOUT_SECONDS":            "20",
		"AI_ANALYSIS_CACHE_TTL_SECONDS": "0",
		"AI_ANALYSIS_CACHE_MAX_ENTRIES": "3",
		"OPENROUTER_API_KEY":            "or-key",
		"OPENROUTER_MODEL":              "vendor/model",
		"OPENROUTER_SERVER_URL":         "http://127.0.0.1:9000/v1",
		"OPENROUTER_APP_TITLE":          "Collector",
		"OPENROUTER_HTTP_REFERER":       "http://127.0.0.1:8080",
		"OPENCODE_ZEN_API_KEY":          "<your-opencode-zen-key>",
		"OPENCODE_ZEN_MODEL":            "opencode/big-pickle",
		"OPENCODE_ZEN_SERVER_URL":       "http://127.0.0.1:9001/v1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := AI{
		Providers:       []string{"openrouter", "opencode_zen", "mock"},
		Timeout:         20 * time.Second,
		CacheTTL:        0,
		CacheMaxEntries: 3,
		OpenRouter: ai.ChatConfig{Name: "OpenRouter", ServerURL: "http://127.0.0.1:9000/v1", APIKey: "or-key", Model: "vendor/model",
			Headers: map[string]string{"HTTP-Referer": "http://127.0.0.1:8080", "X-OpenRouter-Title": "Collector"}},
		OpenCodeZen: ai.ChatConfig{Name: "OpenCode Zen", ServerURL: "http://127.0.0.1:9001/v1", Model: "big-pickle"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestLoadAIRejectsBadValues(t *testing.T) {
	for _, tt := range []struct{ key, value, want string }{
		{"AI_PROVIDERS", "openrouter,openai", `AI_PROVIDERS names "openai"; use openrouter, opencode_zen, or mock`},
		{"AI_TIMEOUT_SECONDS", "0", `AI_TIMEOUT_SECONDS "0" is not a positive number of seconds`},
		{"AI_TIMEOUT_SECONDS", "soon", `AI_TIMEOUT_SECONDS "soon" is not a positive number of seconds`},
		{"AI_ANALYSIS_CACHE_TTL_SECONDS", "-1", `AI_ANALYSIS_CACHE_TTL_SECONDS "-1" is not a number of seconds, 0 or more`},
		{"AI_ANALYSIS_CACHE_MAX_ENTRIES", "many", `AI_ANALYSIS_CACHE_MAX_ENTRIES "many" is not a number of entries, 0 or more`},
	} {
		_, err := LoadAI(env(map[string]string{tt.key: tt.value}))
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s=%q: error = %v, want %q", tt.key, tt.value, err, tt.want)
		}
	}
}

func TestLoadAPIRefusesAnUnknownProvider(t *testing.T) {
	vars := validAPIEnv()
	vars["AI_PROVIDERS"] = "openai"
	if _, err := LoadAPI(env(vars)); err == nil || !strings.Contains(err.Error(), `"openai"`) {
		t.Errorf("error = %v, want one naming openai", err)
	}
}
