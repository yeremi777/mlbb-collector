//go:build integration

// Package dbtest opens the database for integration tests.
package dbtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/config"
)

// BeginTx opens a transaction on the DB_* database that is rolled back when
// the test ends, so no test leaves a row behind.
func BeginTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	url, err := config.DatabaseURL()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(ctx)
		_ = conn.Close(ctx)
	})
	return ctx, tx
}
