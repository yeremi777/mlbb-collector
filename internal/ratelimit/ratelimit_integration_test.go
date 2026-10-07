//go:build integration

package ratelimit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var testRedis *redis.Options

// TestMain points the tests at database TEST_REDIS_DB on the REDIS_HOST and
// REDIS_PORT server, 127.0.0.1:6379 when unset, and exits when REDIS_DB names
// that database.
func TestMain(m *testing.M) {
	testDB, err := strconv.Atoi(os.Getenv("TEST_REDIS_DB"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "TEST_REDIS_DB is not a database number; run make test-integration")
		os.Exit(1)
	}
	if appDB, _ := strconv.Atoi(os.Getenv("REDIS_DB")); testDB == appDB {
		fmt.Fprintf(os.Stderr, "refusing to empty Redis database %d: REDIS_DB names it\n", testDB)
		os.Exit(1)
	}
	host, port := os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "6379"
	}
	testRedis = &redis.Options{Addr: net.JoinHostPort(host, port), DB: testDB}
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
	CookieName: "mlbb_collector_client_id", CookieMaxAge: 2592000, CookieSameSite: http.SameSiteLaxMode,
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

// enforce returns the cookies Enforce issued and its outcome.
func enforce(l *Limiter, route string, r *http.Request) ([]*http.Cookie, error) {
	w := httptest.NewRecorder()
	err := l.Enforce(w, r, route)
	return w.Result().Cookies(), err
}

func wantExceeded(t *testing.T, err error, windowSeconds int) {
	t.Helper()
	var exceeded *ExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("error = %v, want *ExceededError", err)
	}
	if exceeded.RetryAfter < 1 || exceeded.RetryAfter > windowSeconds {
		t.Errorf("RetryAfter = %d, want 1 to %d", exceeded.RetryAfter, windowSeconds)
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
			if _, err := enforce(l, tt.route, request("10.0.0.1", "")); err != nil {
				t.Fatalf("%s request %d: %v", tt.route, i+1, err)
			}
		}
		cookies, err := enforce(l, tt.route, request("10.0.0.1", ""))
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
		if _, err := enforce(l, "analyze-counter-score", request(fmt.Sprintf("10.0.1.%d", i), browser)); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	_, err := enforce(l, "analyze-counter-score", request("10.0.1.99", browser))
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
		cookies, err := enforce(l, "analyze-counter-score", request("10.0.2.1", cookie))
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
		cookies, err := enforce(l, "analyze-counter-detail", request("10.0.3.1", cookie))
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

// devGolden is a request sequence and what dev's limiter at b04727f answered
// and left in Redis for it, with each issued browser ID as <issued-N>.
type devGolden struct {
	Requests []struct {
		Route        string `json:"route"`
		RemoteAddr   string `json:"remoteAddr"`
		ForwardedFor string `json:"forwardedFor"`
		Cookie       string `json:"cookie"`
	} `json:"requests"`
	Responses []goldenResponse `json:"responses"`
	Redis     []goldenCounter  `json:"redis"`
}

type goldenResponse struct {
	Outcome    string `json:"outcome"`
	RetryAfter int    `json:"retryAfter"`
	SetCookie  string `json:"setCookie"`
}

type goldenCounter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
}

// within1 allows a clock tick between dev's capture and this run.
func within1(got, want int) bool { return got >= want-1 && got <= want }

func TestRedisMatchesDev(t *testing.T) {
	raw, err := os.ReadFile("testdata/dev.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden devGolden
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	client := emptyRedis(t)
	l := newLimiter(t, Config{
		MaxRequests: 2, WindowSeconds: 60, DetailMultiplier: 3,
		CookieName: "mlbb_analyzer_client_id", CookieMaxAge: 3600, CookieSameSite: http.SameSiteLaxMode,
		Salt: "golden-salt",
	})

	var issued []string
	placeholders := func(s string) string {
		for i, id := range issued {
			s = strings.ReplaceAll(s, id, fmt.Sprintf("<issued-%d>", i+1))
		}
		return s
	}
	for i, q := range golden.Requests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = q.RemoteAddr
		if q.ForwardedFor != "" {
			r.Header.Set("X-Forwarded-For", q.ForwardedFor)
		}
		if q.Cookie != "" {
			r.AddCookie(&http.Cookie{Name: "mlbb_analyzer_client_id", Value: q.Cookie})
		}
		w := httptest.NewRecorder()
		got := goldenResponse{Outcome: "allowed"}
		var exceeded *ExceededError
		switch err := l.Enforce(w, r, q.Route); {
		case errors.As(err, &exceeded):
			got.Outcome, got.RetryAfter = "exceeded", exceeded.RetryAfter
		case err != nil:
			got.Outcome = "unavailable"
		}
		if cookies := w.Result().Cookies(); len(cookies) == 1 {
			issued = append(issued, cookies[0].Value)
		}
		got.SetCookie = placeholders(w.Header().Get("Set-Cookie"))
		want := golden.Responses[i]
		if got.Outcome != want.Outcome || got.SetCookie != want.SetCookie || !within1(got.RetryAfter, want.RetryAfter) {
			t.Errorf("request %d %+v: got %+v, want %+v", i+1, q, got, want)
		}
	}

	keys, err := client.Keys(context.Background(), "*").Result()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]goldenCounter{}
	for _, key := range keys {
		value, err := client.Get(context.Background(), key).Result()
		if err != nil {
			t.Fatal(err)
		}
		ttl, err := client.TTL(context.Background(), key).Result()
		if err != nil {
			t.Fatal(err)
		}
		got[placeholders(key)] = goldenCounter{placeholders(key), value, int(ttl.Seconds())}
	}
	for _, want := range golden.Redis {
		c, ok := got[want.Key]
		if !ok || c.Value != want.Value || !within1(c.TTL, want.TTL) {
			t.Errorf("%s: got %+v, want %+v", want.Key, c, want)
		}
		delete(got, want.Key)
	}
	for key := range got {
		t.Errorf("%s is in Redis but not in dev's", key)
	}
}
