package ingest

import (
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}
