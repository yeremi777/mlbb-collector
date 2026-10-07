package config

import (
	"testing"
)

func TestLoadRedisDefaults(t *testing.T) {
	got, err := LoadRedis(env(map[string]string{"REDIS_HOST": "127.0.0.1"}))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Redis{Host: "127.0.0.1", Port: 6379, DB: 0}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRedisOptions(t *testing.T) {
	for _, tt := range []struct {
		vars       map[string]string
		addr       string
		databaseNo int
	}{
		{map[string]string{"REDIS_HOST": "redis.internal", "REDIS_PORT": "6380", "REDIS_DB": "2"}, "redis.internal:6380", 2},
		{map[string]string{"REDIS_HOST": "::1"}, "[::1]:6379", 0},
	} {
		r, err := LoadRedis(env(tt.vars))
		if err != nil {
			t.Fatal(err)
		}
		if opts := r.Options(); opts.Addr != tt.addr || opts.DB != tt.databaseNo {
			t.Errorf("%v: addr %q db %d, want %q db %d", tt.vars, opts.Addr, opts.DB, tt.addr, tt.databaseNo)
		}
	}
}

func TestLoadRedisRejectsMissingOrBadValues(t *testing.T) {
	for _, tt := range []struct{ key, value, want string }{
		{"REDIS_HOST", "", "REDIS_HOST is not set; rate limiting needs it"},
		{"REDIS_PORT", "63x9", `REDIS_PORT "63x9" is not a port number`},
		{"REDIS_PORT", "70000", `REDIS_PORT "70000" is not a port number`},
		{"REDIS_DB", "-1", `REDIS_DB "-1" is not a database number, 0 or more`},
		{"REDIS_DB", "cache", `REDIS_DB "cache" is not a database number, 0 or more`},
	} {
		vars := map[string]string{"REDIS_HOST": "127.0.0.1"}
		vars[tt.key] = tt.value
		_, err := LoadRedis(env(vars))
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s=%q: error = %v, want %q", tt.key, tt.value, err, tt.want)
		}
	}
}
