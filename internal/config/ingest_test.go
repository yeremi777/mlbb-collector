package config

import (
	"strings"
	"testing"
)

func TestLoadIngestDefaults(t *testing.T) {
	got, err := LoadIngest(env(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Location.String() != "Asia/Jakarta" {
		t.Errorf("location = %s, want Asia/Jakarta", got.Location)
	}
	if got.UserAgent != DefaultIngestUserAgent || !strings.Contains(got.UserAgent, "@") {
		t.Errorf("user agent = %q, want the default with contact details", got.UserAgent)
	}
	if got.MoontonBaseURL != "https://api.gms.moontontech.com/api/gms/source/2669606" {
		t.Errorf("moonton = %q", got.MoontonBaseURL)
	}
	if got.LiquipediaBaseURL != "https://liquipedia.net/mobilelegends/api.php" {
		t.Errorf("liquipedia = %q", got.LiquipediaBaseURL)
	}
}

func TestLoadIngestOverrides(t *testing.T) {
	got, err := LoadIngest(env(map[string]string{
		"INGEST_TIMEZONE":     "UTC",
		"INGEST_USER_AGENT":   "probe/1.0 (me@example.com)",
		"MOONTON_BASE_URL":    "http://127.0.0.1:9000/source",
		"LIQUIPEDIA_BASE_URL": "http://127.0.0.1:1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Location.String() != "UTC" || got.UserAgent != "probe/1.0 (me@example.com)" ||
		got.MoontonBaseURL != "http://127.0.0.1:9000/source" || got.LiquipediaBaseURL != "http://127.0.0.1:1" {
		t.Errorf("got %+v", got)
	}
}

func TestLoadIngestRefusesBadValues(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"INGEST_TIMEZONE", "Mars/Olympus"},
		{"MOONTON_BASE_URL", "api.gms.moontontech.com"},
		{"MOONTON_BASE_URL", "ftp://example.com"},
		{"LIQUIPEDIA_BASE_URL", "http://"},
	} {
		_, err := LoadIngest(env(map[string]string{tc.key: tc.value}))
		if err == nil || !strings.Contains(err.Error(), tc.key) {
			t.Errorf("%s=%q: err = %v, want one naming %s", tc.key, tc.value, err, tc.key)
		}
	}
}
