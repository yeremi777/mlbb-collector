//go:build integration

// Package dbtest runs integration tests against a test database, never the
// .env one.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

var conn *pgx.Conn

// Main rebuilds the schema of the database named by TEST_DB_DSN from the
// migrations, runs the package's tests, empties the database again so no
// table outlives the run, and exits. It refuses any database whose name does
// not start with "test".
func Main(m *testing.M) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "TEST_DB_DSN is not set; run make test-integration")
		os.Exit(1)
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var name string
	if err := c.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil || !strings.HasPrefix(name, "test") {
		fmt.Fprintf(os.Stderr, "refusing to reset database %q: its name must start with \"test\" (%v)\n", name, err)
		os.Exit(1)
	}
	code := 1
	if err := applyMigrations(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		conn = c
		code = m.Run()
	}
	if err := emptySchema(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, "empty the test database:", err)
		code = 1
	}
	c.Close(ctx)
	os.Exit(code)
}

// emptySchema drops every table, leaving an empty public schema.
func emptySchema(ctx context.Context, c *pgx.Conn) error {
	_, err := c.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	return err
}

// applyMigrations empties the schema and runs the goose Up section of every
// migration, oldest first.
func applyMigrations(ctx context.Context, c *pgx.Conn) error {
	if err := emptySchema(ctx, c); err != nil {
		return err
	}
	_, here, _, _ := runtime.Caller(0)
	files, err := filepath.Glob(filepath.Join(filepath.Dir(here), "..", "migrations", "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found next to %s", here)
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		up, _, ok := strings.Cut(string(b), "-- +goose Down")
		if !ok {
			return fmt.Errorf("%s has no Down section", f)
		}
		if _, err := c.PgConn().Exec(ctx, up).ReadAll(); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

// BeginTx opens a transaction on the test database that is rolled back when
// the test ends, so each test starts from the empty migrated schema.
func BeginTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return ctx, tx
}
