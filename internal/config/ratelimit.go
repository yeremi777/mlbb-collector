package config

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/yeremi777/mlbb-collector/internal/ratelimit"
)

// RateLimit is whether analyze requests are limited, and the limiter's
// settings. A disabled one is the zero value.
type RateLimit struct {
	Enabled bool
	Limiter ratelimit.Config
}

// LoadRateLimit reads RATE_LIMIT_ENABLED and, only when it is true, the other
// RATE_LIMIT_* variables. RATE_LIMIT_SALT is then required.
func LoadRateLimit(getenv func(string) string) (RateLimit, error) {
	enabled, err := flag(getenv, "RATE_LIMIT_ENABLED")
	if err != nil || !enabled {
		return RateLimit{}, err
	}
	if getenv("RATE_LIMIT_SALT") == "" {
		return RateLimit{}, fmt.Errorf("RATE_LIMIT_SALT is not set; rate limiting needs it")
	}
	cfg := ratelimit.Config{CookieName: or(getenv("RATE_LIMIT_COOKIE_NAME"), "mlbb_collector_client_id"), Salt: getenv("RATE_LIMIT_SALT")}
	for _, n := range []struct {
		key      string
		fallback int
		into     *int
	}{
		{"RATE_LIMIT_ANALYZE_MAX_REQUESTS", 5, &cfg.MaxRequests},
		{"RATE_LIMIT_ANALYZE_WINDOW_SECONDS", 18000, &cfg.WindowSeconds},
		{"RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER", 3, &cfg.DetailMultiplier},
		{"RATE_LIMIT_COOKIE_MAX_AGE_SECONDS", 2592000, &cfg.CookieMaxAge},
	} {
		if *n.into, err = intOr(getenv, n.key, n.fallback, 1, "a whole number of at least 1"); err != nil {
			return RateLimit{}, err
		}
	}
	if cfg.CookieSecure, err = flag(getenv, "RATE_LIMIT_COOKIE_SECURE"); err != nil {
		return RateLimit{}, err
	}
	switch sameSite := strings.ToLower(or(getenv("RATE_LIMIT_COOKIE_SAMESITE"), "lax")); sameSite {
	case "lax":
		cfg.CookieSameSite = http.SameSiteLaxMode
	case "strict":
		cfg.CookieSameSite = http.SameSiteStrictMode
	case "none":
		if !cfg.CookieSecure {
			return RateLimit{}, fmt.Errorf("RATE_LIMIT_COOKIE_SAMESITE=none needs RATE_LIMIT_COOKIE_SECURE=true; browsers drop the cookie otherwise")
		}
		cfg.CookieSameSite = http.SameSiteNoneMode
	default:
		return RateLimit{}, fmt.Errorf("RATE_LIMIT_COOKIE_SAMESITE %q is not lax, strict, or none", getenv("RATE_LIMIT_COOKIE_SAMESITE"))
	}
	return RateLimit{Enabled: true, Limiter: cfg}, nil
}

// flag reads key as true or false, false when unset.
func flag(getenv func(string) string, key string) (bool, error) {
	switch raw := getenv(key); raw {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s %q is not true or false", key, raw)
	}
}
