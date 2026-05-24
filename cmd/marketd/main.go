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
	"github.com/lzqqdy/marketpulse/internal/logging"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/server"
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

	marketData := marketdata.New(cfg)
	marketData.Start(ctx)

	srv := server.New(server.Deps{
		Config:     cfg,
		MarketData: marketData,
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
