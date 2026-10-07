package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// API is what the API server needs: its database, listen port and URL, browser
// origins, AI and rate-limit settings, the Redis that holds the counters, and
// how long a response may take.
type API struct {
	Database        Database
	Port            int
	URL             string
	FrontendOrigins []string
	AI              AI
	RateLimit       RateLimit
	Redis           Redis
	// WriteTimeout bounds writing one response: the AI deadline plus margin,
	// so an analysis answer is never cut off.
	WriteTimeout time.Duration
}

// LoadAPI reads APP_PORT and APP_URL, both required, FRONTEND_ORIGIN, which may
// be empty, and the variables LoadDatabase, LoadAI, LoadRateLimit, and, only
// when rate limiting is enabled, LoadRedis read.
func LoadAPI(getenv func(string) string) (API, error) {
	db, err := LoadDatabase(getenv)
	if err != nil {
		return API{}, err
	}
	for _, key := range []string{"APP_PORT", "APP_URL"} {
		if getenv(key) == "" {
			return API{}, fmt.Errorf("%s is not set; copy .env.example to .env", key)
		}
	}
	port, err := strconv.Atoi(getenv("APP_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return API{}, fmt.Errorf("APP_PORT %q is not a port number", getenv("APP_PORT"))
	}
	appURL := getenv("APP_URL")
	u, err := url.Parse(appURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return API{}, fmt.Errorf("APP_URL %q is not an absolute http or https URL", appURL)
	}
	// A URL that names a port is this server itself, so it must be the port
	// the server listens on; one without a port is a proxy in front of it.
	if p := u.Port(); p != "" && p != strconv.Itoa(port) {
		return API{}, fmt.Errorf("APP_URL %q names port %s, but APP_PORT is %d", appURL, p, port)
	}
	aiConfig, err := LoadAI(getenv)
	if err != nil {
		return API{}, err
	}
	rateLimit, err := LoadRateLimit(getenv)
	if err != nil {
		return API{}, err
	}
	var redisConfig Redis
	if rateLimit.Enabled {
		if redisConfig, err = LoadRedis(getenv); err != nil {
			return API{}, err
		}
	}
	var origins []string
	for _, o := range strings.Split(getenv("FRONTEND_ORIGIN"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return API{
		Database:        db,
		Port:            port,
		URL:             appURL,
		FrontendOrigins: origins,
		AI:              aiConfig,
		RateLimit:       rateLimit,
		Redis:           redisConfig,
		WriteTimeout:    aiConfig.Timeout + 10*time.Second,
	}, nil
}
