// Command seed makes the public tables of the database named by the DB_*
// variables match the authored dataset in data/ (ADR-0001). It validates the
// whole dataset before it connects, so an invalid dataset never reaches the
// database.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/config"
	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/seed"
)

const seedTimeout = 2 * time.Minute

func main() {
	dir := flag.String("data", "data", "dataset directory")
	flag.Parse()

	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	db, err := config.LoadDatabase(os.Getenv)
	if err != nil {
		return err
	}
	ds, err := dataset.Load(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), seedTimeout)
	defer cancel()
	conn, err := database.Connect(ctx, db.DSN())
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error { return seed.Sync(ctx, tx, ds) }); err != nil {
		return err
	}

	var counterProofs, synergyProofs int
	for _, c := range ds.Counters {
		counterProofs += len(c.Proof)
	}
	for _, s := range ds.Synergies {
		synergyProofs += len(s.Proof)
	}
	fmt.Printf("synced heroes %d, counters %d, counter_proofs %d, synergies %d, synergy_proofs %d\n",
		len(ds.Heroes), len(ds.Counters), counterProofs, len(ds.Synergies), synergyProofs)
	return nil
}
