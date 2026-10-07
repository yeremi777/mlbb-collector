// Package config reads the service's settings from the environment, after an
// optional .env file in the working directory.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadDotEnv adds the variables in ./.env to the environment, leaving any
// variable already set untouched. A missing file is not an error.
func LoadDotEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

var sslModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// DatabaseURL assembles the Postgres connection URL from the DB_* variables.
// DB_PASSWORD may be empty; every other one is required.
func DatabaseURL() (string, error) {
	values := map[string]string{}
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USERNAME", "DB_SSLMODE"} {
		values[key] = os.Getenv(key)
		if values[key] == "" {
			return "", fmt.Errorf("%s is required", key)
		}
	}
	if port, err := strconv.Atoi(values["DB_PORT"]); err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("DB_PORT: want a port from 1 to 65535, got %q", values["DB_PORT"])
	}
	if !slices.Contains(sslModes, values["DB_SSLMODE"]) {
		return "", fmt.Errorf("DB_SSLMODE: %q is not one of %s", values["DB_SSLMODE"], strings.Join(sslModes, ", "))
	}

	user := url.User(values["DB_USERNAME"])
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		user = url.UserPassword(values["DB_USERNAME"], password)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     net.JoinHostPort(values["DB_HOST"], values["DB_PORT"]),
		Path:     "/" + values["DB_NAME"],
		RawQuery: url.Values{"sslmode": {values["DB_SSLMODE"]}}.Encode(),
	}
	return u.String(), nil
}

// API is the API server's configuration.
type API struct {
	DatabaseURL     string
	Port            int
	URL             string
	FrontendOrigins []string
	// WriteTimeout bounds writing one response: the AI deadline plus margin,
	// so an analysis answer is never cut off.
	WriteTimeout time.Duration
}

// LoadAPI reads and validates the API server's configuration.
func LoadAPI() (API, error) {
	dbURL, err := DatabaseURL()
	if err != nil {
		return API{}, err
	}

	port := envOr("APP_PORT", "8080")
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return API{}, fmt.Errorf("APP_PORT: want a port from 1 to 65535, got %q", port)
	}

	appURL := envOr("APP_URL", "http://127.0.0.1:8080")
	if u, err := url.Parse(appURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return API{}, fmt.Errorf("APP_URL: want an absolute http or https URL, got %q", appURL)
	}

	aiTimeout := envOr("AI_TIMEOUT_SECONDS", "60")
	seconds, err := strconv.Atoi(aiTimeout)
	if err != nil || seconds < 1 {
		return API{}, fmt.Errorf("AI_TIMEOUT_SECONDS: want a positive number of seconds, got %q", aiTimeout)
	}

	var origins []string
	for _, o := range strings.Split(os.Getenv("FRONTEND_ORIGIN"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}

	return API{
		DatabaseURL:     dbURL,
		Port:            n,
		URL:             appURL,
		FrontendOrigins: origins,
		WriteTimeout:    time.Duration(seconds)*time.Second + 10*time.Second,
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
