// Command api serves the hero catalog, Counters, and Synergies over HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yeremi777/mlbb-collector/docs"
	"github.com/yeremi777/mlbb-collector/internal/config"
	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/httpx"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

// drainTimeout is how long in-flight requests may run after a stop signal.
const drainTimeout = 25 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("api failed", "err", err)
		os.Exit(1)
	}
}

// register adds every route the API serves to mux.
func register(mux httpx.Mux, db database.Querier, spec []byte) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	heroes := hero.NewRepository(db)
	hero.NewHandler(heroes).Register(mux)
	counter.NewHandler(heroes, counter.NewRepository(db)).Register(mux)
	synergy.NewHandler(heroes, synergy.NewRepository(db)).Register(mux)
	httpx.Docs(mux, spec)
}

func run() error {
	if err := config.LoadDotEnv(); err != nil {
		return err
	}
	cfg, err := config.LoadAPI()
	if err != nil {
		return err
	}
	spec, err := docs.SpecFor(cfg.URL)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	mux := http.NewServeMux()
	register(mux, pool, spec)
	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Handler:           httpx.CORS(cfg.FrontendOrigins, httpx.Router(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	stop, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- srv.ListenAndServe() }()
	slog.Info("api listening", "addr", srv.Addr, "docs", cfg.URL+"/docs")

	select {
	case err := <-served:
		return err
	case <-stop.Done():
	}
	drain, cancelDrain := context.WithTimeout(context.Background(), drainTimeout)
	defer cancelDrain()
	if err := srv.Shutdown(drain); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("api stopped")
	return nil
}
