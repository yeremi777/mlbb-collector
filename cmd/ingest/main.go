// Command ingest lands Moonton's hero statistics in raw.hero_rank_snapshots
// and keeps raw.patches equal to Liquipedia's Patch calendar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/config"
	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/patch"
	"github.com/yeremi777/mlbb-collector/internal/snapshot"
)

const usage = `usage: ingest stats [-date YYYY-MM-DD] [-force]
       ingest patches
       ingest all [-date YYYY-MM-DD] [-force]`

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, time.Now, os.Stderr))
}

type command struct {
	name string
	// date is the -date value, the zero time when unset.
	date  time.Time
	force bool
}

func parseArgs(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, errors.New("no command")
	}
	cmd := command{name: args[0]}
	var date string
	fs := flag.NewFlagSet(cmd.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	switch cmd.name {
	case "stats", "all":
		fs.StringVar(&date, "date", "", "")
		fs.BoolVar(&cmd.force, "force", false, "")
	case "patches":
	default:
		return command{}, fmt.Errorf("unknown command %q", cmd.name)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return command{}, err
	}
	if fs.NArg() > 0 {
		return command{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if date != "" {
		d, err := time.Parse(time.DateOnly, date)
		if err != nil {
			return command{}, fmt.Errorf("-date %q is not YYYY-MM-DD", date)
		}
		cmd.date = d
	}
	return cmd, nil
}

// run returns the exit code: 0 when everything run succeeded, 1 when any
// combination or the calendar failed, and 2 on a bad command, flag, or date.
func run(args []string, getenv func(string) string, now func() time.Time, stderr io.Writer) int {
	refuse := func(err error) int {
		fmt.Fprintf(stderr, "ingest: %v\n%s\n", err, usage)
		return 2
	}
	cmd, err := parseArgs(args)
	if err != nil {
		return refuse(err)
	}
	cfg, err := config.LoadIngest(getenv)
	if err != nil {
		slog.Error("ingest failed", "err", err)
		return 1
	}
	t := now().In(cfg.Location)
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	if !cmd.date.IsZero() {
		if cmd.date.After(date) {
			return refuse(fmt.Errorf("-date %s is after today, %s, in %s", cmd.date.Format(time.DateOnly), date.Format(time.DateOnly), cfg.Location))
		}
		date = cmd.date
	}
	db, err := config.LoadDatabase(getenv)
	if err != nil {
		slog.Error("ingest failed", "err", err)
		return 1
	}
	if cfg.MoontonBaseURL != config.DefaultMoontonBaseURL {
		slog.Warn("upstream is not the default", "MOONTON_BASE_URL", cfg.MoontonBaseURL)
	}
	if cfg.LiquipediaBaseURL != config.DefaultLiquipediaBaseURL {
		slog.Warn("upstream is not the default", "LIQUIPEDIA_BASE_URL", cfg.LiquipediaBaseURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A second signal is left to kill the process.
	go func() {
		<-ctx.Done()
		stop()
	}()
	conn, err := database.Connect(ctx, db.DSN())
	if err != nil {
		slog.Error("ingest failed", "err", err)
		return 1
	}
	defer conn.Close(context.Background())

	return ingest(ctx, conn, cmd, date,
		snapshot.NewClient(cfg.MoontonBaseURL, cfg.UserAgent),
		patch.NewClient(cfg.LiquipediaBaseURL, cfg.UserAgent))
}

// ingest runs the command's feeds, the calendar first. A calendar failure is
// logged and statistics still run.
func ingest(ctx context.Context, db database.DB, cmd command, date time.Time, stats *snapshot.Client, patches *patch.Client) int {
	code := 0
	if cmd.name == "patches" || cmd.name == "all" {
		calendar, err := patches.Fetch(ctx)
		changed := 0
		if err == nil {
			changed, err = patch.Sync(ctx, db, calendar)
		}
		if err != nil {
			slog.Error("patches", "fetched", len(calendar), "changed", changed, "err", err)
			code = 1
		} else {
			slog.Info("patches", "fetched", len(calendar), "changed", changed)
		}
	}
	if cmd.name == "stats" || cmd.name == "all" {
		s, err := snapshot.Ingest(ctx, db, stats, date, cmd.force)
		summary := []any{"date", date.Format(time.DateOnly), "fetched", s.Fetched, "skipped", s.Skipped,
			"failed", s.Failed, "inserted", s.Inserted}
		switch {
		case err != nil:
			slog.Error("stats", append(summary, "err", err)...)
			code = 1
		case s.Failed > 0:
			slog.Error("stats", summary...)
			code = 1
		default:
			slog.Info("stats", summary...)
		}
	}
	return code
}
