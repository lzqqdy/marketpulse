package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// mountStatic serves a Vite build (index.html + assets/) when staticDir is set.
// API (/api, /ws) and /healthz are registered before this and keep priority.
func mountStatic(r *gin.Engine, staticDir string) error {
	abs, err := filepath.Abs(staticDir)
	if err != nil {
		return err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return os.ErrNotExist
	}

	indexPath := filepath.Join(abs, "index.html")
	assetsDir := filepath.Join(abs, "assets")

	r.Static("/assets", assetsDir)
	r.GET("/", func(c *gin.Context) {
		c.File(indexPath)
	})

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		p := c.Request.URL.Path
		if isReservedPath(p) {
			c.Status(http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(p, "/")
		if rel == "" {
			c.File(indexPath)
			return
		}
		fp := filepath.Join(abs, filepath.Clean(rel))
		if !strings.HasPrefix(fp, abs+string(filepath.Separator)) && fp != abs {
			c.Status(http.StatusNotFound)
			return
		}
		if st, err := os.Stat(fp); err == nil && !st.IsDir() {
			c.File(fp)
			return
		}
		c.File(indexPath)
	})
	return nil
}

func isReservedPath(p string) bool {
	return p == "/healthz" ||
		strings.HasPrefix(p, "/api") ||
		strings.HasPrefix(p, "/ws") ||
		strings.HasPrefix(p, "/uploads")
}
