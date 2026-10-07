// Command seed makes the public tables match the authored dataset in data/
// (ADR-0001). It loads and validates the whole dataset before it connects, so
// an invalid dataset never reaches the database.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/config"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/seed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	dir := flags.String("data", "data", "dataset directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.LoadDotEnv(); err != nil {
		return err
	}

	ds, err := dataset.Load(*dir)
	if err != nil {
		return err
	}
	url, err := config.DatabaseURL()
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)
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
