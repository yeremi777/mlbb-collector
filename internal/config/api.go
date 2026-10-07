package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// API is what the API server needs: its database, where it listens and is
// reached, the browser origins it serves, and how long a response may take.
type API struct {
	Database        Database
	Port            int
	URL             string
	FrontendOrigins []string
	// WriteTimeout bounds writing one response: the AI deadline plus margin,
	// so an analysis answer is never cut off.
	WriteTimeout time.Duration
}

// defaultAITimeout applies while AI_TIMEOUT_SECONDS is unset.
const defaultAITimeout = 60

// LoadAPI reads the DB_* and APP_* variables, FRONTEND_ORIGIN, and
// AI_TIMEOUT_SECONDS. FRONTEND_ORIGIN may be empty and AI_TIMEOUT_SECONDS
// unset; every other variable is required.
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
	aiTimeout := defaultAITimeout
	if raw := getenv("AI_TIMEOUT_SECONDS"); raw != "" {
		if aiTimeout, err = strconv.Atoi(raw); err != nil || aiTimeout < 1 {
			return API{}, fmt.Errorf("AI_TIMEOUT_SECONDS %q is not a positive number of seconds", raw)
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
		WriteTimeout:    time.Duration(aiTimeout)*time.Second + 10*time.Second,
	}, nil
}
