package ingest

import (
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/baidu"
)

// BaiduConfigFrom converts application config into Baidu ingest settings.
func BaiduConfigFrom(cfg *config.Config) baidu.Config {
	if cfg == nil {
		return baidu.Config{Enabled: true, WSEnabled: true}
	}
	return baidu.Config{
		Enabled:          cfg.Ingest.Baidu.IsEnabled(),
		BaseURL:          cfg.Ingest.Baidu.BaseURL,
		WSURL:            cfg.Ingest.Baidu.WSURL,
		WSEnabled:        cfg.Ingest.Baidu.IsWSEnabled(),
		WSReconnectMax:   cfg.Ingest.Baidu.WSReconnectMax,
		WSReconnectDelay: int(cfg.Ingest.Baidu.WSReconnectDelay / 1e9),
		WSPatchInterval:  int(cfg.Ingest.Baidu.WSPatchInterval / 1e9),
	}
}
