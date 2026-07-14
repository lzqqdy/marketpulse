// Package main is the MarketPulse market data service entrypoint.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/logging"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformmysql "github.com/lzqqdy/marketpulse/internal/platform/mysql"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
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

	var db *sql.DB
	if cfg.MySQL.Enabled {
		db, err = platformmysql.Open(cfg.MySQL)
		if err != nil {
			slog.Error("open mysql", "err", err)
			os.Exit(1)
		}
		defer db.Close()
		slog.Info("mysql connected", "host", cfg.MySQL.Host, "database", cfg.MySQL.Database)
	}

	var rdb *platformredis.Client
	if cfg.Redis.Enabled {
		rdb, err = platformredis.Open(cfg.Redis)
		if err != nil {
			slog.Error("open redis", "err", err)
			os.Exit(1)
		}
		defer rdb.Close()
		slog.Info("redis connected", "addr", cfg.Redis.Addr, "db", cfg.Redis.DB)
	}

	marketData := marketdata.New(cfg)
	marketData.Start(ctx)

	srv := server.New(server.Deps{
		Config:     cfg,
		MarketData: marketData,
		MySQL:      db,
		Redis:      rdb,
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
