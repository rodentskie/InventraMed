package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/logger"
	"go.uber.org/zap"

	"apps/ws/config"
	wshandler "apps/ws/internal/handler/ws"
	"apps/ws/internal/hub"
)

const readHeaderTimeout = 10 * time.Second

func main() {
	log := logger.New()
	defer log.Sync() //nolint:errcheck

	if err := run(log); err != nil {
		log.Fatal("ws server stopped", zap.Error(err))
	}
}

// run serves until SIGINT/SIGTERM, then stops accepting connections and
// closes every open socket.
func run(log *zap.Logger) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := hub.New(log)
	go h.Run()
	// Shutdown does not close hijacked WebSocket connections; the hub does.
	defer h.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", wshandler.NewHandler(h, cfg.AllowedOrigins, log).Serve)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("starting ws server", zap.String("port", cfg.Port), zap.Strings("allowed_origins", cfg.AllowedOrigins))
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down ws server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}
