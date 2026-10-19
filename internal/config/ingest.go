package config

import (
	"fmt"
	"net/url"
	"time"
)

// The upstream defaults. A run with another base URL is pointed at a stub.
const (
	DefaultMoontonBaseURL    = "https://api.gms.moontontech.com/api/gms/source/2669606"
	DefaultLiquipediaBaseURL = "https://liquipedia.net/mobilelegends/api.php"
	// DefaultIngestUserAgent carries contact details, as Liquipedia's API terms require.
	DefaultIngestUserAgent = "mlbb-collector/1.0 (+github.com/yeremi777/mlbb-collector; kuroganehunter99@gmail.com)"
)

// Ingest is what cmd/ingest needs beyond the database.
type Ingest struct {
	// Location is the zone whose today is a run's date.
	Location          *time.Location
	UserAgent         string
	MoontonBaseURL    string
	LiquipediaBaseURL string
}

// LoadIngest reads INGEST_TIMEZONE, INGEST_USER_AGENT, MOONTON_BASE_URL, and
// LIQUIPEDIA_BASE_URL; each has a default.
func LoadIngest(getenv func(string) string) (Ingest, error) {
	zone := or(getenv("INGEST_TIMEZONE"), "Asia/Jakarta")
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return Ingest{}, fmt.Errorf("INGEST_TIMEZONE %q is not a time zone", zone)
	}
	cfg := Ingest{
		Location:          loc,
		UserAgent:         or(getenv("INGEST_USER_AGENT"), DefaultIngestUserAgent),
		MoontonBaseURL:    or(getenv("MOONTON_BASE_URL"), DefaultMoontonBaseURL),
		LiquipediaBaseURL: or(getenv("LIQUIPEDIA_BASE_URL"), DefaultLiquipediaBaseURL),
	}
	for _, u := range []struct{ key, value string }{
		{"MOONTON_BASE_URL", cfg.MoontonBaseURL},
		{"LIQUIPEDIA_BASE_URL", cfg.LiquipediaBaseURL},
	} {
		parsed, err := url.Parse(u.value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Ingest{}, fmt.Errorf("%s %q is not an http or https URL", u.key, u.value)
		}
	}
	return cfg, nil
}
