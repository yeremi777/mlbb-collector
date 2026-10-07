package config

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DB_HOST": "127.0.0.1", "DB_PORT": "5432", "DB_NAME": "mlbb_collector",
		"DB_USERNAME": "postgres", "DB_PASSWORD": "", "DB_SSLMODE": "disable",
	}
}

// pgx's own parser is the judge of what the DSN means.
func TestDatabaseDSNRoundTripsThroughPgx(t *testing.T) {
	for _, password := range []string{"", "root", "p w", `it's`, `back\slash`, `'; sslmode=require`} {
		vars := validEnv()
		vars["DB_PASSWORD"] = password
		db, err := LoadDatabase(env(vars))
		if err != nil {
			t.Fatalf("password %q: %v", password, err)
		}
		dsn := db.DSN()
		cfg, err := pgconn.ParseConfig(dsn)
		if err != nil {
			t.Fatalf("password %q: pgx cannot parse %q: %v", password, dsn, err)
		}
		if cfg.Password != password || cfg.Host != "127.0.0.1" || cfg.Port != 5432 ||
			cfg.Database != "mlbb_collector" || cfg.User != "postgres" || cfg.TLSConfig != nil {
			t.Errorf("password %q parsed as password=%q host=%q port=%d db=%q user=%q tls=%v",
				password, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.TLSConfig != nil)
		}
	}
}

func TestLoadDatabaseRejectsMissingOrBadValues(t *testing.T) {
	for _, tt := range []struct{ key, value, want string }{
		{"DB_HOST", "", "DB_HOST is not set"},
		{"DB_NAME", "", "DB_NAME is not set"},
		{"DB_USERNAME", "", "DB_USERNAME is not set"},
		{"DB_SSLMODE", "", "DB_SSLMODE is not set"},
		{"DB_PORT", "", "DB_PORT is not set"},
		{"DB_PORT", "54x2", `DB_PORT "54x2" is not a port number`},
		{"DB_PORT", "70000", `DB_PORT "70000" is not a port number`},
	} {
		vars := validEnv()
		vars[tt.key] = tt.value
		_, err := LoadDatabase(env(vars))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s=%q: error = %v, want %q", tt.key, tt.value, err, tt.want)
		}
	}
}
