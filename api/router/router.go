package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/example/edge-rollout-control/api/handler"
	"github.com/example/edge-rollout-control/api/middleware"
	"github.com/example/edge-rollout-control/internal/system"
)

func New(handler *handler.Handler, logger *slog.Logger, ids system.IDGenerator, bodyLimit int64, timeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	handler.Register(mux)
	limiter := middleware.NewRateLimiter(600, time.Minute)
	return middleware.Chain(
		mux,
		middleware.RequestID(ids),
		middleware.Recovery(logger),
		middleware.AccessLog(logger),
		middleware.SecurityHeaders(),
		middleware.CORS(nil),
		limiter.Middleware(),
		middleware.BodyLimit(bodyLimit),
		func(next http.Handler) http.Handler { return middleware.Actor(next) },
		middleware.Timeout(timeout),
	)
}
