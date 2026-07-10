package baidu

import (
	"net/http"
	"net/url"
)

// GetAPI performs a Baidu Finance GET with CDN fallback.
func GetAPI(client *http.Client, cfg Config, path string, query url.Values) (APIResponse, error) {
	cfg = normalizeConfig(cfg)
	return getJSON(client, cfg.BaseURL, path, query)
}
