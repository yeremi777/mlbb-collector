// Package ratelimit counts analyze requests per client address and per
// browser in Redis, over a fixed rate-limit window.
package ratelimit

import "net/http"

// Config is how many requests a counter allows, for how long, and the
// browser cookie that names a client.
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
