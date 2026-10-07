package config

import (
	"fmt"
	"net"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Redis is the Redis database the REDIS_* variables describe.
type Redis struct {
	Host string
	Port int
	DB   int
}

// LoadRedis reads the REDIS_* variables. REDIS_HOST is required; REDIS_PORT
// defaults to 6379 and REDIS_DB to 0.
func LoadRedis(getenv func(string) string) (Redis, error) {
	host := getenv("REDIS_HOST")
	if host == "" {
		return Redis{}, fmt.Errorf("REDIS_HOST is not set; rate limiting needs it")
	}
	port, err := intOr(getenv, "REDIS_PORT", 6379, 1, "a port number")
	if err != nil {
		return Redis{}, err
	}
	if port > 65535 {
		return Redis{}, fmt.Errorf("REDIS_PORT %q is not a port number", getenv("REDIS_PORT"))
	}
	db, err := intOr(getenv, "REDIS_DB", 0, 0, "a database number, 0 or more")
	if err != nil {
		return Redis{}, err
	}
	return Redis{Host: host, Port: port, DB: db}, nil
}

// Options dial r's host and port and select database r.DB.
func (r Redis) Options() *redis.Options {
	return &redis.Options{Addr: net.JoinHostPort(r.Host, strconv.Itoa(r.Port)), DB: r.DB}
}
