package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/ai"
)

// AI is how analysis asks AI providers: the provider names in fall-through
// order, the deadline of one request's provider work, the result cache, and
// each chat provider's settings.
type AI struct {
	Providers       []string
	Timeout         time.Duration
	CacheTTL        time.Duration
	CacheMaxEntries int
	OpenRouter      ai.ChatConfig
	OpenCodeZen     ai.ChatConfig
}

// providerNames are the names AI_PROVIDERS may list.
var providerNames = []string{"openrouter", "opencode_zen", "mock"}

// LoadAI reads the AI_*, OPENROUTER_*, and OPENCODE_ZEN_* variables, every one
// optional, and refuses a provider name outside providerNames.
func LoadAI(getenv func(string) string) (AI, error) {
	var providers []string
	for _, name := range strings.Split(getenv("AI_PROVIDERS"), ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if !slices.Contains(providerNames, name) {
			return AI{}, fmt.Errorf("AI_PROVIDERS names %q; use openrouter, opencode_zen, or mock", name)
		}
		providers = append(providers, name)
	}
	timeout, err := seconds(getenv, "AI_TIMEOUT_SECONDS", 60, 1, "a positive number of seconds")
	if err != nil {
		return AI{}, err
	}
	ttl, err := seconds(getenv, "AI_ANALYSIS_CACHE_TTL_SECONDS", 600, 0, "a number of seconds, 0 or more")
	if err != nil {
		return AI{}, err
	}
	maxEntries, err := intOr(getenv, "AI_ANALYSIS_CACHE_MAX_ENTRIES", 256, 0, "a number of entries, 0 or more")
	if err != nil {
		return AI{}, err
	}
	return AI{
		Providers:       providers,
		Timeout:         timeout,
		CacheTTL:        ttl,
		CacheMaxEntries: maxEntries,
		OpenRouter: ai.ChatConfig{
			Name:      "OpenRouter",
			ServerURL: or(getenv("OPENROUTER_SERVER_URL"), "https://openrouter.ai/api/v1"),
			APIKey:    apiKey(getenv("OPENROUTER_API_KEY")),
			Model:     or(getenv("OPENROUTER_MODEL"), "openrouter/free"),
			Headers: map[string]string{
				"HTTP-Referer":       or(getenv("OPENROUTER_HTTP_REFERER"), "http://127.0.0.1:8000"),
				"X-OpenRouter-Title": or(getenv("OPENROUTER_APP_TITLE"), "MLBB Analyzer Service"),
			},
		},
		OpenCodeZen: ai.ChatConfig{
			Name:      "OpenCode Zen",
			ServerURL: or(getenv("OPENCODE_ZEN_SERVER_URL"), "https://opencode.ai/zen/v1"),
			APIKey:    apiKey(getenv("OPENCODE_ZEN_API_KEY")),
			Model:     strings.TrimPrefix(getenv("OPENCODE_ZEN_MODEL"), "opencode/"),
		},
	}, nil
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// apiKey treats a placeholder such as <your-key> as no key.
func apiKey(key string) string {
	if strings.HasPrefix(key, "<") {
		return ""
	}
	return key
}

// intOr reads key as an integer of at least lowest, or fallback when unset.
func intOr(getenv func(string) string, key string, fallback, lowest int, want string) (int, error) {
	raw := getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lowest {
		return 0, fmt.Errorf("%s %q is not %s", key, raw, want)
	}
	return n, nil
}

// seconds is intOr read as a duration in seconds.
func seconds(getenv func(string) string, key string, fallback, lowest int, want string) (time.Duration, error) {
	n, err := intOr(getenv, key, fallback, lowest, want)
	return time.Duration(n) * time.Second, err
}
