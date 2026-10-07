// Command api serves the hero catalog, Counters, and Synergies over HTTP, and
// scores and explains them with an AI provider.
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

	"github.com/yeremi777/mlbb-collector/docs"
	"github.com/yeremi777/mlbb-collector/internal/analysis"
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

// register adds every route the API serves to mux. A nil analyzer means no AI
// provider is configured.
func register(mux httpx.Mux, db database.Querier, analyzer *analysis.Analyzer, spec []byte) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	heroes := hero.NewRepository(db)
	hero.NewHandler(heroes).Register(mux)
	counters, synergies := counter.NewRepository(db), synergy.NewRepository(db)
	counter.NewHandler(heroes, counters).Register(mux)
	synergy.NewHandler(heroes, synergies).Register(mux)
	analysis.NewHandler(heroes, counters, synergies, analyzer).Register(mux)
	httpx.Docs(mux, spec)
}

func run() error {
	cfg, err := config.LoadAPI(os.Getenv)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := database.OpenPool(ctx, cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer pool.Close()

	analyzer := analysis.NewFromConfig(cfg.AI)
	if analyzer == nil {
		slog.Warn("no AI provider is usable; the analyze routes answer ai_provider_not_configured", "AI_PROVIDERS", cfg.AI.Providers)
	}
	mux := http.NewServeMux()
	register(mux, pool, analyzer, docs.Spec())
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
	slog.Info("api listening", "url", cfg.URL, "docs", cfg.URL+"/docs")

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
