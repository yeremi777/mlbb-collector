//go:build integration

package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var testRedis *redis.Options

// TestMain reads the test Redis from TEST_REDIS_URL and refuses database 0,
// the one REDIS_URL names by default.
func TestMain(m *testing.M) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		fmt.Fprintln(os.Stderr, "TEST_REDIS_URL is not set; run make test-integration")
		os.Exit(1)
	}
	opts, err := redis.ParseURL(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "TEST_REDIS_URL:", err)
		os.Exit(1)
	}
	if opts.DB == 0 {
		fmt.Fprintf(os.Stderr, "refusing to empty Redis database 0 of %s: name another database\n", raw)
		os.Exit(1)
	}
	testRedis = opts
	os.Exit(m.Run())
}

// emptyRedis empties the test database and returns a client to inspect it.
func emptyRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(testRedis)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	return client
}

// defaultConfig is the .env.example quota: 5 score and 15 detail requests.
var defaultConfig = Config{
	MaxRequests: 5, WindowSeconds: 18000, DetailMultiplier: 3,
	CookieName: "mlbb_analyzer_client_id", CookieMaxAge: 2592000, CookieSameSite: http.SameSiteLaxMode,
	Salt: "test-salt",
}

func newLimiter(t *testing.T, cfg Config) *Limiter {
	t.Helper()
	l := New(testRedis, cfg)
	t.Cleanup(func() { _ = l.Close() })
	return l
}

// request is a POST from address carrying cookie, when not empty, as the
// browser cookie.
func request(address, cookie string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("X-Forwarded-For", address)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: defaultConfig.CookieName, Value: cookie})
	}
	return r
}

// enforce returns the outcome and the cookies Enforce issued.
func enforce(l *Limiter, route string, r *http.Request) (error, []*http.Cookie) {
	w := httptest.NewRecorder()
	err := l.Enforce(w, r, route)
	return err, w.Result().Cookies()
}

func wantExceeded(t *testing.T, err error, window int) {
	t.Helper()
	var exceeded *ExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("error = %v, want *ExceededError", err)
	}
	if exceeded.RetryAfter < 1 || exceeded.RetryAfter > window {
		t.Errorf("RetryAfter = %d, want 1 to %d", exceeded.RetryAfter, window)
	}
}

func counter(t *testing.T, client *redis.Client, key string) string {
	t.Helper()
	value, err := client.Get(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("GET %s: %v", key, err)
	}
	return value
}

func TestAnAddressOverItsQuotaIsRefused(t *testing.T) {
	for _, tt := range []struct {
		route   string
		allowed int
	}{{"analyze-counter-score", 5}, {"analyze-synergy-detail", 15}} {
		client := emptyRedis(t)
		l := newLimiter(t, defaultConfig)
		for i := range tt.allowed {
			if err, _ := enforce(l, tt.route, request("10.0.0.1", "")); err != nil {
				t.Fatalf("%s request %d: %v", tt.route, i+1, err)
			}
		}
		err, cookies := enforce(l, tt.route, request("10.0.0.1", ""))
		wantExceeded(t, err, defaultConfig.WindowSeconds)
		if len(cookies) != 0 {
			t.Errorf("%s: a request refused by the address check got cookies %v", tt.route, cookies)
		}
		ipKey := "rate:analyze:" + tt.route + ":ip:" + l.hash("10.0.0.1")
		if got := counter(t, client, ipKey); got != fmt.Sprint(tt.allowed) {
			t.Errorf("%s: address counter = %s after a refusal, want %d", tt.route, got, tt.allowed)
		}
	}
}

func TestABrowserOverItsQuotaIsRefusedFromANewAddress(t *testing.T) {
	client := emptyRedis(t)
	l := newLimiter(t, defaultConfig)
	browser := uuid.NewString()
	for i := range 5 {
		if err, _ := enforce(l, "analyze-counter-score", request(fmt.Sprintf("10.0.1.%d", i), browser)); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	err, _ := enforce(l, "analyze-counter-score", request("10.0.1.99", browser))
	wantExceeded(t, err, defaultConfig.WindowSeconds)
	if got := counter(t, client, "rate:analyze:analyze-counter-score:client:"+browser); got != "5" {
		t.Errorf("browser counter = %s after a refusal, want 5", got)
	}
	if got := counter(t, client, "rate:analyze:analyze-counter-score:ip:"+l.hash("10.0.1.99")); got != "1" {
		t.Errorf("address counter = %s, want 1: the browser refusal counts against the address", got)
	}
}

func TestABrowserWithoutAValidCookieIsIssuedOne(t *testing.T) {
	emptyRedis(t)
	cfg := defaultConfig
	cfg.CookieSecure, cfg.CookieSameSite = true, http.SameSiteNoneMode
	l := newLimiter(t, cfg)
	for _, cookie := range []string{"", "not-a-uuid"} {
		err, cookies := enforce(l, "analyze-counter-score", request("10.0.2.1", cookie))
		if err != nil {
			t.Fatal(err)
		}
		if len(cookies) != 1 {
			t.Fatalf("cookie %q: issued %v, want one cookie", cookie, cookies)
		}
		c := cookies[0]
		if _, err := uuid.Parse(c.Value); err != nil || c.Name != cfg.CookieName || c.Path != "/" || c.MaxAge != cfg.CookieMaxAge ||
			!c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteNoneMode {
			t.Errorf("cookie %q: issued %+v", cookie, c)
		}
	}
}

func TestAValidCookieIsKeptAndKeyedCanonically(t *testing.T) {
	client := emptyRedis(t)
	l := newLimiter(t, defaultConfig)
	browser := uuid.New()
	for _, cookie := range []string{browser.String(), "{" + browser.String() + "}", "urn:uuid:" + browser.String()} {
		err, cookies := enforce(l, "analyze-counter-detail", request("10.0.3.1", cookie))
		if err != nil {
			t.Fatal(err)
		}
		if len(cookies) != 0 {
			t.Errorf("cookie %q: issued %v, want none", cookie, cookies)
		}
	}
	if got := counter(t, client, "rate:analyze:analyze-counter-detail:client:"+browser.String()); got != "3" {
		t.Errorf("browser counter = %s, want 3", got)
	}
}
