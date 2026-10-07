package ratelimit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
)

var testConfig = Config{
	MaxRequests: 2, WindowSeconds: 60, DetailMultiplier: 3,
	CookieName: "mlbb_analyzer_client_id", CookieMaxAge: 3600, CookieSameSite: http.SameSiteLaxMode,
	Salt: "test-salt",
}

func TestANilLimiterAllowsWithoutACookie(t *testing.T) {
	var l *Limiter
	w := httptest.NewRecorder()
	if err := l.Enforce(w, httptest.NewRequest(http.MethodPost, "/", nil), "analyze-counter-score"); err != nil {
		t.Fatal(err)
	}
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("cookies = %v, want none", cookies)
	}
}

func TestAnUnreachableRedisRefuses(t *testing.T) {
	l := New(&redis.Options{Addr: "127.0.0.1:1"}, testConfig)
	t.Cleanup(func() { _ = l.Close() })
	w := httptest.NewRecorder()
	err := l.Enforce(w, httptest.NewRequest(http.MethodPost, "/", nil), "analyze-counter-score")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("cookies = %v, want none", cookies)
	}
}
