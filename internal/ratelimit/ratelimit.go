// Package ratelimit counts analyze requests per client address and per
// browser in Redis, over a fixed rate-limit window.
package ratelimit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Config is a counter's quota and rate-limit window, the browser cookie's
// settings, and the salt that hashes addresses.
type Config struct {
	MaxRequests      int
	WindowSeconds    int
	DetailMultiplier int
	CookieName       string
	CookieMaxAge     int
	CookieSecure     bool
	CookieSameSite   http.SameSite
	Salt             string
}

// ErrUnavailable is a Redis failure; the request is refused, never let through.
var ErrUnavailable = errors.New("rate limit storage is unavailable")

// ExceededError is a request over its quota. RetryAfter is the refusing
// counter's remaining seconds.
type ExceededError struct{ RetryAfter int }

func (e *ExceededError) Error() string {
	return fmt.Sprintf("rate limit exceeded; retry after %ds", e.RetryAfter)
}

// checkAndIncrement refuses at the limit, else increments, starting the
// rate-limit window on the first request. It answers {allowed, count, ttl}.
var checkAndIncrement = redis.NewScript(`
local count = redis.call("GET", KEYS[1])
if count and tonumber(count) >= tonumber(ARGV[1]) then
    return {0, tonumber(count), redis.call("TTL", KEYS[1])}
end

count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[2])
end

return {1, count, redis.call("TTL", KEYS[1])}
`)

// Limiter counts requests in Redis. A nil Limiter allows every request.
type Limiter struct {
	client *redis.Client
	cfg    Config
}

// New limits with cfg, keeping the counters in the Redis redisOptions names.
// No connection is opened until the first request.
func New(redisOptions *redis.Options, cfg Config) *Limiter {
	return &Limiter{client: redis.NewClient(redisOptions), cfg: cfg}
}

// Close closes the Redis connections.
func (l *Limiter) Close() error { return l.client.Close() }

// Enforce counts r against route's address counter, then its browser counter,
// issuing a browser cookie once the address check passes and r carries no
// valid one. It returns *ExceededError or ErrUnavailable when r is refused.
func (l *Limiter) Enforce(w http.ResponseWriter, r *http.Request, route string) error {
	if l == nil {
		return nil
	}
	maxRequests := l.cfg.MaxRequests
	if strings.HasSuffix(route, "-detail") {
		maxRequests *= l.cfg.DetailMultiplier
	}
	if err := l.increment(r, "rate:analyze:"+route+":ip:"+l.hash(clientAddress(r)), maxRequests); err != nil {
		return err
	}
	clientID := l.clientID(r)
	if clientID == "" {
		clientID = uuid.NewString()
		http.SetCookie(w, &http.Cookie{
			Name:     l.cfg.CookieName,
			Value:    clientID,
			Path:     "/",
			MaxAge:   l.cfg.CookieMaxAge,
			HttpOnly: true,
			Secure:   l.cfg.CookieSecure,
			SameSite: l.cfg.CookieSameSite,
		})
	}
	return l.increment(r, "rate:analyze:"+route+":client:"+clientID, maxRequests)
}

// increment counts r under key, refusing with *ExceededError at maxRequests
// or ErrUnavailable when Redis fails.
func (l *Limiter) increment(r *http.Request, key string, maxRequests int) error {
	result, err := checkAndIncrement.Run(r.Context(), l.client, []string{key}, maxRequests, l.cfg.WindowSeconds).Int64Slice()
	if err != nil || len(result) != 3 {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if result[0] == 0 {
		return &ExceededError{RetryAfter: int(result[2])}
	}
	return nil
}

// clientID is the canonical UUID in r's browser cookie, or empty when it has
// no valid one.
func (l *Limiter) clientID(r *http.Request) string {
	cookie, err := r.Cookie(l.cfg.CookieName)
	if err != nil {
		return ""
	}
	id, err := uuid.Parse(cookie.Value)
	if err != nil {
		return ""
	}
	return id.String()
}

// hash is the hex HMAC-SHA256 of address keyed with the salt.
func (l *Limiter) hash(address string) string {
	mac := hmac.New(sha256.New, []byte(l.cfg.Salt))
	mac.Write([]byte(address))
	return hex.EncodeToString(mac.Sum(nil))
}

// clientAddress is the first X-Forwarded-For entry, else the peer address,
// else "unknown".
func clientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}
