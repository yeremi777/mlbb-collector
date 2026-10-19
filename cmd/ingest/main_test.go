package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// noDatabase points the DB_* variables where nothing listens, so a run that
// gets past its arguments fails to connect and exits 1.
func noDatabase(key string) string {
	return map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "1", "DB_NAME": "none",
		"DB_USERNAME": "postgres", "DB_SSLMODE": "disable", "INGEST_TIMEZONE": "Asia/Jakarta"}[key]
}

// fixedNow is 2026-10-19 00:30 in Jakarta, still 2026-10-18 in UTC.
func fixedNow() time.Time { return time.Date(2026, time.October, 18, 17, 30, 0, 0, time.UTC) }

func TestRunRefusesABadCommandWithUsage(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"backfill"},
		{"stats", "-verbose"},
		{"stats", "extra"},
		{"stats", "-date", "19-10-2026"},
		{"patches", "-force"},
		{"stats", "-date", "2026-10-20"},
		{"all", "-date", "2999-01-01", "-force"},
	} {
		var stderr bytes.Buffer
		if code := run(args, noDatabase, fixedNow, &stderr); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: ingest") {
			t.Errorf("%q: stderr %q, want the usage text", args, stderr.String())
		}
	}
}

func TestRunAcceptsTodayInTheIngestTimezone(t *testing.T) {
	for _, args := range [][]string{
		{"stats", "-date", "2026-10-19"},
		{"all", "-date", "2026-10-01", "-force"},
		{"patches"},
		{"stats"},
	} {
		var stderr bytes.Buffer
		if code := run(args, noDatabase, fixedNow, &stderr); code != 1 || strings.Contains(stderr.String(), "usage") {
			t.Errorf("%q: exit %d, stderr %q; want 1 from the unreachable database", args, code, stderr.String())
		}
	}
}

func TestRunRefusesAMalformedDateBeforeReadingConfig(t *testing.T) {
	badZone := func(key string) string { return map[string]string{"INGEST_TIMEZONE": "Mars/Olympus"}[key] }
	var stderr bytes.Buffer
	if code := run([]string{"stats", "-date", "19-10-2026"}, badZone, fixedNow, &stderr); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
