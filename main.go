package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/boredape874/Better-pm-AC/proxy"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	p := proxy.New(cfg, log)
	if err := p.ListenAndServe(ctx); err != nil {
		log.Error("proxy error", "err", err)
		os.Exit(1)
	}

	log.Info("proxy shut down")
}
