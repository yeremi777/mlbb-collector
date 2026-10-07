// Package database holds the schema migrations and the query surface the
// repositories share.
package database

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is what a repository reads through: a pool in the service, a
// transaction in tests.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
