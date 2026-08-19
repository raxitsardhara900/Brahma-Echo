package server

import (
	"time"

	"github.com/pinchtab/pinchtab/internal/httpx"
)

const (
	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 30 * time.Second
	serverWriteTimeout      = httpx.MaxNavigationHTTPDuration
	serverIdleTimeout       = 120 * time.Second
)
