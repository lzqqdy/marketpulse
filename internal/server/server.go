package server

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/api"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/logging"
)

// Server wraps the Gin engine and dependencies.
type Server struct {
	engine *gin.Engine
	cfg    *config.Config
}

// New builds a configured Gin server.
func New(deps Deps) *Server {
	cfg := deps.Config
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.RecoveryWithWriter(logging.ErrorWriter()))
	r.Use(corsMiddleware(cfg))

	h := &api.Handler{
		Config:     cfg,
		MarketData: deps.MarketData,
		Users:      deps.Users,
		StartedAt:  time.Now().UTC(),
	}
	api.Register(r, h)

	if deps.Upload != nil && deps.Upload.Dir() != "" {
		public := deps.Upload.PublicPath()
		r.Static(public, deps.Upload.Dir())
		slog.Info("serving uploads", "path", public, "dir", deps.Upload.Dir())
	}

	if dir := cfg.App.StaticDir; dir != "" {
		if err := mountStatic(r, dir); err != nil {
			slog.Warn("static dir unavailable, skipping SPA", "dir", dir, "err", err)
		} else {
			slog.Info("serving frontend", "dir", dir)
		}
	}

	return &Server{engine: r, cfg: cfg}
}

// Run starts the HTTP listener.
func (s *Server) Run() error {
	return s.engine.Run(s.cfg.App.Addr)
}

// Engine exposes the Gin engine for tests.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// AddrLabel returns a human-readable listen address for logs.
func AddrLabel(cfg *config.Config) string {
	return fmt.Sprintf("%s (mode=%s)", cfg.App.Addr, cfg.App.Mode)
}

func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.CORS.AllowedOrigins))
	for _, o := range cfg.CORS.AllowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok || cfg.App.Mode == "debug" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Token, Upgrade, Connection")
				c.Header("Vary", "Origin")
			}
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
