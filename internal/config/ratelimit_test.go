package config

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/ratelimit"
)

func validRateLimitEnv() map[string]string {
	return map[string]string{
		"RATE_LIMIT_ENABLED": "true",
		"REDIS_URL":          "redis://127.0.0.1:6379/15",
		"RATE_LIMIT_SALT":    "salt",
	}
}

func TestLoadRateLimitDisabledReadsNothingElse(t *testing.T) {
	for _, enabled := range []string{"", "false"} {
		got, err := LoadRateLimit(env(map[string]string{
			"RATE_LIMIT_ENABLED":              enabled,
			"REDIS_URL":                       "::bad",
			"RATE_LIMIT_ANALYZE_MAX_REQUESTS": "many",
		}))
		if err != nil {
			t.Fatalf("RATE_LIMIT_ENABLED=%q: %v", enabled, err)
		}
		if !reflect.DeepEqual(got, RateLimit{}) {
			t.Errorf("RATE_LIMIT_ENABLED=%q: got %+v, want disabled", enabled, got)
		}
	}
}

func TestLoadRateLimitDefaults(t *testing.T) {
	got, err := LoadRateLimit(env(validRateLimitEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Redis == nil || got.Redis.Addr != "127.0.0.1:6379" || got.Redis.DB != 15 {
		t.Errorf("got %+v, redis %+v", got, got.Redis)
	}
	want := ratelimit.Config{
		MaxRequests: 5, WindowSeconds: 18000, DetailMultiplier: 3,
		CookieName: "mlbb_analyzer_client_id", CookieMaxAge: 2592000, CookieSameSite: http.SameSiteLaxMode,
		Salt: "salt",
	}
	if got.Limiter != want {
		t.Errorf("limiter %+v\nwant    %+v", got.Limiter, want)
	}
}

func TestLoadRateLimit(t *testing.T) {
	vars := validRateLimitEnv()
	for k, v := range map[string]string{
		"RATE_LIMIT_ANALYZE_MAX_REQUESTS":      "2",
		"RATE_LIMIT_ANALYZE_WINDOW_SECONDS":    "60",
		"RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER": "4",
		"RATE_LIMIT_COOKIE_NAME":               "client",
		"RATE_LIMIT_COOKIE_MAX_AGE_SECONDS":    "3600",
		"RATE_LIMIT_COOKIE_SECURE":             "true",
		"RATE_LIMIT_COOKIE_SAMESITE":           "None",
	} {
		vars[k] = v
	}
	got, err := LoadRateLimit(env(vars))
	if err != nil {
		t.Fatal(err)
	}
	want := ratelimit.Config{
		MaxRequests: 2, WindowSeconds: 60, DetailMultiplier: 4,
		CookieName: "client", CookieMaxAge: 3600, CookieSecure: true, CookieSameSite: http.SameSiteNoneMode,
		Salt: "salt",
	}
	if got.Limiter != want {
		t.Errorf("limiter %+v\nwant    %+v", got.Limiter, want)
	}
}

func TestLoadRateLimitRejectsBadValues(t *testing.T) {
	for _, tt := range []struct{ key, value, want string }{
		{"RATE_LIMIT_ENABLED", "yes", `RATE_LIMIT_ENABLED "yes" is not true or false`},
		{"REDIS_URL", "", "REDIS_URL is not set; rate limiting needs it"},
		{"REDIS_URL", "127.0.0.1:6379", `REDIS_URL "127.0.0.1:6379" is not a redis:// or rediss:// URL`},
		{"RATE_LIMIT_SALT", "", "RATE_LIMIT_SALT is not set; rate limiting needs it"},
		{"RATE_LIMIT_ANALYZE_MAX_REQUESTS", "0", `RATE_LIMIT_ANALYZE_MAX_REQUESTS "0" is not a whole number of at least 1`},
		{"RATE_LIMIT_ANALYZE_WINDOW_SECONDS", "soon", `RATE_LIMIT_ANALYZE_WINDOW_SECONDS "soon" is not a whole number of at least 1`},
		{"RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER", "0", `RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER "0" is not a whole number of at least 1`},
		{"RATE_LIMIT_COOKIE_MAX_AGE_SECONDS", "-1", `RATE_LIMIT_COOKIE_MAX_AGE_SECONDS "-1" is not a whole number of at least 1`},
		{"RATE_LIMIT_COOKIE_SECURE", "1", `RATE_LIMIT_COOKIE_SECURE "1" is not true or false`},
		{"RATE_LIMIT_COOKIE_SAMESITE", "loose", `RATE_LIMIT_COOKIE_SAMESITE "loose" is not lax, strict, or none`},
		{"RATE_LIMIT_COOKIE_SAMESITE", "none", "RATE_LIMIT_COOKIE_SAMESITE=none needs RATE_LIMIT_COOKIE_SECURE=true; browsers drop the cookie otherwise"},
	} {
		vars := validRateLimitEnv()
		vars[tt.key] = tt.value
		_, err := LoadRateLimit(env(vars))
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s=%q: error = %v, want %q", tt.key, tt.value, err, tt.want)
		}
	}
}

func TestLoadAPIReadsTheRateLimit(t *testing.T) {
	vars := validAPIEnv()
	vars["RATE_LIMIT_ENABLED"] = "true"
	vars["REDIS_URL"] = "redis://127.0.0.1:6379/15"
	if _, err := LoadAPI(env(vars)); err == nil || !strings.Contains(err.Error(), "RATE_LIMIT_SALT") {
		t.Errorf("error = %v, want one naming RATE_LIMIT_SALT", err)
	}
}
