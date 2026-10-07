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
