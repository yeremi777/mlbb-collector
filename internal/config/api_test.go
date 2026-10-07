package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func validAPIEnv() map[string]string {
	vars := validEnv()
	vars["APP_PORT"] = "8080"
	vars["APP_URL"] = "http://127.0.0.1:8080"
	return vars
}

func TestLoadAPI(t *testing.T) {
	vars := validAPIEnv()
	vars["FRONTEND_ORIGIN"] = " https://a.example , ,https://b.example"
	got, err := LoadAPI(env(vars))
	if err != nil {
		t.Fatal(err)
	}
	db, _ := LoadDatabase(env(vars))
	aiConfig, _ := LoadAI(env(vars))
	want := API{
		Database:        db,
		Port:            8080,
		URL:             "http://127.0.0.1:8080",
		FrontendOrigins: []string{"https://a.example", "https://b.example"},
		AI:              aiConfig,
		WriteTimeout:    70 * time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}

	vars["APP_URL"] = "https://api.example"
	if got, err := LoadAPI(env(vars)); err != nil || got.URL != "https://api.example" {
		t.Errorf("APP_URL without a port: got %q, %v; want it accepted", got.URL, err)
	}

	vars["AI_TIMEOUT_SECONDS"] = "20"
	if got, err := LoadAPI(env(vars)); err != nil || got.WriteTimeout != 30*time.Second {
		t.Errorf("AI_TIMEOUT_SECONDS=20: write timeout %v, %v; want 30s", got.WriteTimeout, err)
	}
}

func TestLoadAPIRejectsMissingOrBadValues(t *testing.T) {
	for _, tt := range []struct{ key, value, want string }{
		{"DB_HOST", "", "DB_HOST is not set"},
		{"APP_PORT", "", "APP_PORT is not set; copy .env.example to .env"},
		{"APP_URL", "", "APP_URL is not set; copy .env.example to .env"},
		{"APP_PORT", "abc", `APP_PORT "abc" is not a port number`},
		{"APP_PORT", "0", `APP_PORT "0" is not a port number`},
		{"APP_URL", "localhost", `APP_URL "localhost" is not an absolute http or https URL`},
		{"APP_URL", "ftp://api.example", `APP_URL "ftp://api.example" is not an absolute http or https URL`},
		{"AI_TIMEOUT_SECONDS", "0", `AI_TIMEOUT_SECONDS "0" is not a positive number of seconds`},
		{"APP_URL", "http://127.0.0.1:8082", `APP_URL "http://127.0.0.1:8082" names port 8082, but APP_PORT is 8080`},
	} {
		vars := validAPIEnv()
		vars[tt.key] = tt.value
		_, err := LoadAPI(env(vars))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s=%q: error = %v, want %q", tt.key, tt.value, err, tt.want)
		}
	}
}
