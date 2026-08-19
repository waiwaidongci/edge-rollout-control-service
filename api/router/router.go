package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/edge-rollout-control/api/handler"
	"github.com/example/edge-rollout-control/api/middleware"
	"github.com/example/edge-rollout-control/internal/system"
)

func RunRouterWork(ctx context.Context, work func(context.Context) error) error {
	return middleware.RunWithRequestContext(ctx, work)
}

func RouterContextError(ctx context.Context) error {
	return middleware.RequestContextError(ctx)
}

func RouterContextState(ctx context.Context) string {
	return middleware.RequestContextState(ctx)
}

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
