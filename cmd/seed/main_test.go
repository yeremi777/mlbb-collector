package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRejectsAnInvalidDatasetBeforeConnecting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "heroes.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Nothing listens here: reaching for the database would fail differently.
	for k, v := range map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "1", "DB_NAME": "none",
		"DB_USERNAME": "postgres", "DB_PASSWORD": "", "DB_SSLMODE": "disable"} {
		t.Setenv(k, v)
	}

	err := run(dir)
	if err == nil || err.Error() != "heroes.json: no heroes" {
		t.Errorf("err = %v, want the dataset error alone", err)
	}
}
