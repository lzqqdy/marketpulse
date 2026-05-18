// Package main is the MarketPulse market data service entrypoint.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/hub"
	"github.com/lzqqdy/marketpulse/internal/ingest"
	"github.com/lzqqdy/marketpulse/internal/logging"
	"github.com/lzqqdy/marketpulse/internal/server"
	"github.com/lzqqdy/marketpulse/internal/store"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "path", *configPath, "err", err)
		os.Exit(1)
	}
	if err := logging.Setup(cfg.App.LogDir); err != nil {
		slog.Error("setup logging", "dir", cfg.App.LogDir, "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.New(cfg.Symbols...)
	streamHub := hub.NewStreamHub(st)
	klineHub := hub.NewKlineHub(cfg)
	ingestSvc := ingest.New(cfg, st)
	ingestSvc.Start(ctx)

	srv := server.New(server.Deps{
		Config:    cfg,
		Store:     st,
		StreamHub: streamHub,
		KlineHub:  klineHub,
		Ingest:    ingestSvc,
	})

	slog.Info("marketpulse marketd listening", "addr", server.AddrLabel(cfg))
	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
	}()

	if err := srv.Run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
