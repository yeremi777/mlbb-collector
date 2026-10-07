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
	"github.com/yeremi777/mlbb-collector/internal/ratelimit"
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
// provider is configured, and a nil limiter limits nothing.
func register(mux httpx.Mux, db database.Querier, analyzer *analysis.Analyzer, limiter *ratelimit.Limiter, spec []byte) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	heroes := hero.NewRepository(db)
	hero.NewHandler(heroes).Register(mux)
	counters, synergies := counter.NewRepository(db), synergy.NewRepository(db)
	counter.NewHandler(heroes, counters).Register(mux)
	synergy.NewHandler(heroes, synergies).Register(mux)
	analysis.NewHandler(heroes, counters, synergies, analyzer, limiter).Register(mux)
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
	var limiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		limiter = ratelimit.New(cfg.Redis.Options(), cfg.RateLimit.Limiter)
		defer limiter.Close()
	}
	mux := http.NewServeMux()
	register(mux, pool, analyzer, limiter, docs.Spec())
	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Handler:           httpx.CORS(cfg.FrontendOrigins, httpx.Router(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// Bind before logging, so "api listening" is printed only once the port
	// is ours.
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	stop, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
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
