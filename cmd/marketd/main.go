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
	"github.com/lzqqdy/marketpulse/internal/alerts"
	"github.com/lzqqdy/marketpulse/internal/ai"
	"github.com/lzqqdy/marketpulse/internal/portfolio"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformmysql "github.com/lzqqdy/marketpulse/internal/platform/mysql"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/platform/upload"
	"github.com/lzqqdy/marketpulse/internal/server"
	"github.com/lzqqdy/marketpulse/internal/users"
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
	if cfg.UsersSkipReason != "" {
		slog.Warn("users module skipped", "reason", cfg.UsersSkipReason)
	}
	if cfg.AlertsSkipReason != "" {
		slog.Warn("alerts module skipped", "reason", cfg.AlertsSkipReason)
	}
	if cfg.PortfolioSkipReason != "" {
		slog.Warn("portfolio module skipped", "reason", cfg.PortfolioSkipReason)
	}
	if cfg.AISkipReason != "" {
		slog.Warn("ai module skipped", "reason", cfg.AISkipReason)
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

	uploadStore, err := upload.New(cfg.Upload)
	if err != nil {
		slog.Error("init upload store", "err", err)
		os.Exit(1)
	}
	slog.Info("upload store ready", "dir", uploadStore.Dir(), "public", uploadStore.PublicPath())

	var userSvc users.Service
	if cfg.Users.Enabled {
		userSvc, err = users.Bootstrap(ctx, users.BootstrapArgs{
			Users:  cfg.Users,
			DB:     db,
			Redis:  rdb,
			Upload: uploadStore,
		})
		if err != nil {
			slog.Error("bootstrap users", "err", err)
			os.Exit(1)
		}
		slog.Info("users module enabled", "auto_migrate", cfg.Users.IsAutoMigrate())
	}

	marketData := marketdata.New(cfg)
	marketData.Start(ctx)

	var alertSvc alerts.Service
	var alertStream *alerts.StreamServer
	var portfolioSvc portfolio.Service
	if cfg.Alerts.Enabled {
		alertSvc, err = alerts.Bootstrap(ctx, alerts.BootstrapArgs{
			Alerts:     cfg.Alerts,
			SMTP:       cfg.SMTP,
			DB:         db,
			Redis:      rdb,
			MarketData: marketData,
			Users:      userSvc,
		})
		if err != nil {
			slog.Error("bootstrap alerts", "err", err)
			os.Exit(1)
		}
		if alertSvc != nil && alertSvc.Enabled() {
			alertStream = alerts.NewStreamServer(alertSvc, userSvc)
			slog.Info("alerts module enabled")
		}
	}

	if cfg.Portfolio.Enabled {
		portfolioSvc, err = portfolio.Bootstrap(ctx, portfolio.BootstrapArgs{
			Portfolio:  cfg.Portfolio,
			DB:         db,
			MarketData: marketData,
			Users:      userSvc,
		})
		if err != nil {
			slog.Error("bootstrap portfolio", "err", err)
			os.Exit(1)
		}
		if portfolioSvc != nil && portfolioSvc.Enabled() {
			slog.Info("portfolio module enabled")
		}
	}

	var aiSvc ai.Service
	if cfg.AI.Enabled {
		aiSvc, err = ai.Bootstrap(ctx, ai.BootstrapArgs{
			AI:         cfg.AI,
			DB:         db,
			Redis:      rdb,
			MarketData: marketData,
			Users:      userSvc,
		})
		if err != nil {
			slog.Error("bootstrap ai", "err", err)
			os.Exit(1)
		}
		if aiSvc != nil && aiSvc.Enabled() {
			slog.Info("ai module enabled", "model", cfg.AI.Model)
		}
	}

	srv := server.New(server.Deps{
		Config:      cfg,
		MarketData:  marketData,
		Users:       userSvc,
		Alerts:      alertSvc,
		AlertStream: alertStream,
		Portfolio:   portfolioSvc,
		AI:          aiSvc,
		Upload:      uploadStore,
		MySQL:       db,
		Redis:       rdb,
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
