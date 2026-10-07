package config

import "testing"

func setDB(t *testing.T, overrides map[string]string) {
	t.Helper()
	values := map[string]string{
		"DB_HOST": "127.0.0.1", "DB_PORT": "5432", "DB_NAME": "mlbb_collector",
		"DB_USERNAME": "postgres", "DB_PASSWORD": "p@ss:word/1", "DB_SSLMODE": "disable",
	}
	for k, v := range overrides {
		values[k] = v
	}
	for k, v := range values {
		t.Setenv(k, v)
	}
}

func TestDatabaseURL(t *testing.T) {
	setDB(t, nil)
	got, err := DatabaseURL()
	want := "postgres://postgres:p%40ss%3Aword%2F1@127.0.0.1:5432/mlbb_collector?sslmode=disable"
	if err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}

	setDB(t, map[string]string{"DB_PASSWORD": ""})
	if got, err := DatabaseURL(); err != nil || got != "postgres://postgres@127.0.0.1:5432/mlbb_collector?sslmode=disable" {
		t.Errorf("empty password: got %q, %v", got, err)
	}
}

func TestDatabaseURLRejectsBadSettings(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]string
		want      string
	}{
		{"host unset", map[string]string{"DB_HOST": ""}, "DB_HOST is required"},
		{"name unset", map[string]string{"DB_NAME": ""}, "DB_NAME is required"},
		{"username unset", map[string]string{"DB_USERNAME": ""}, "DB_USERNAME is required"},
		{"port unset", map[string]string{"DB_PORT": ""}, "DB_PORT is required"},
		{"port not a number", map[string]string{"DB_PORT": "abc"}, `DB_PORT: want a port from 1 to 65535, got "abc"`},
		{"port out of range", map[string]string{"DB_PORT": "70000"}, `DB_PORT: want a port from 1 to 65535, got "70000"`},
		{"sslmode unset", map[string]string{"DB_SSLMODE": ""}, "DB_SSLMODE is required"},
		{"sslmode unknown", map[string]string{"DB_SSLMODE": "off"}, `DB_SSLMODE: "off" is not one of disable, allow, prefer, require, verify-ca, verify-full`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setDB(t, c.overrides)
			if _, err := DatabaseURL(); err == nil || err.Error() != c.want {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}
