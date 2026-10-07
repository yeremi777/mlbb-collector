// Package config reads and checks the environment variables the commands need.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Database is the Postgres connection the DB_* variables describe.
type Database struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// LoadDatabase reads the DB_* variables. DB_PASSWORD may be empty; every other variable is required.
func LoadDatabase(getenv func(string) string) (Database, error) {
	db := Database{
		Host:     getenv("DB_HOST"),
		Name:     getenv("DB_NAME"),
		User:     getenv("DB_USERNAME"),
		Password: getenv("DB_PASSWORD"),
		SSLMode:  getenv("DB_SSLMODE"),
	}
	for _, v := range []struct{ key, value string }{
		{"DB_HOST", db.Host}, {"DB_PORT", getenv("DB_PORT")}, {"DB_NAME", db.Name},
		{"DB_USERNAME", db.User}, {"DB_SSLMODE", db.SSLMode},
	} {
		if v.value == "" {
			return Database{}, fmt.Errorf("%s is not set; copy .env.example to .env", v.key)
		}
	}
	port, err := strconv.Atoi(getenv("DB_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return Database{}, fmt.Errorf("DB_PORT %q is not a port number", getenv("DB_PORT"))
	}
	db.Port = port
	return db, nil
}

// DSN is the keyword/value connection string; every value is quoted so spaces,
// quotes, and backslashes survive parsing.
func (d Database) DSN() string {
	return strings.Join([]string{
		"host=" + quote(d.Host),
		"port=" + quote(strconv.Itoa(d.Port)),
		"dbname=" + quote(d.Name),
		"user=" + quote(d.User),
		"password=" + quote(d.Password),
		"sslmode=" + quote(d.SSLMode),
	}, " ")
}

func quote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return "'" + value + "'"
}
