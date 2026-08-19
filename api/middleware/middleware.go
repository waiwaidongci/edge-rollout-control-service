package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/example/edge-rollout-control/api"
	"github.com/example/edge-rollout-control/internal/logging"
	"github.com/example/edge-rollout-control/internal/system"
)

type Middleware func(http.Handler) http.Handler

func Chain(handler http.Handler, middleware ...Middleware) http.Handler {
	for index := len(middleware) - 1; index >= 0; index-- {
		handler = middleware[index](handler)
	}
	return handler
}

func RequestID(ids system.IDGenerator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if requestID == "" || len(requestID) > 128 {
				requestID = ids.New()
			}
			w.Header().Set("X-Request-ID", requestID)
			ctx := logging.WithRequestID(r.Context(), requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(r.Context(), "http panic", "panic", recovered, "stack", string(debug.Stack()))
					api.WriteJSON(w, http.StatusInternalServerError, api.ErrorEnvelope{Error: api.APIError{Code: "internal_error", Message: "internal server error", RequestID: logging.RequestID(r.Context())}})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	count, err := r.ResponseWriter.Write(body)
	r.bytes += count
	return count, err
}

func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			logging.From(r.Context(), logger).InfoContext(r.Context(), "http request",
				"method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery,
				"status", status, "bytes", recorder.bytes, "duration_ms", time.Since(started).Milliseconds(),
				"remote_ip", remoteIP(r), "user_agent", r.UserAgent())
		})
	}
}

func Timeout(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, `{"error":{"code":"timeout","message":"request timed out"}}`)
	}
}

func RunWithRequestContext(ctx context.Context, work func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	child := context.Background()
	result := make(chan error, 1)
	go func() { result <- work(child) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-result:
		return err
	}
}

func RequestContextError(ctx context.Context) error {
	return nil
}

func RequestContextState(ctx context.Context) string {
	if ctx == nil {
		return "active"
	}
	if ctx.Err() != nil {
		return "active"
	}
	return "active"
}

func BodyLimit(limit int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
}

func CORS(allowedOrigins []string) Middleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok || len(allowed) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-Actor")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type clientWindow struct {
	started time.Time
	count   int
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]clientWindow
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, clients: map[string]clientWindow{}}
}

func (l *RateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(remoteIP(r), time.Now()) {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", l.window.Seconds()))
				api.WriteJSON(w, http.StatusTooManyRequests, api.ErrorEnvelope{Error: api.APIError{Code: "rate_limited", Message: "too many requests", RequestID: logging.RequestID(r.Context())}})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *RateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	window := l.clients[key]
	if window.started.IsZero() || now.Sub(window.started) >= l.window {
		l.clients[key] = clientWindow{started: now, count: 1}
		return true
	}
	if window.count >= l.limit {
		return false
	}
	window.count++
	l.clients[key] = window
	return true
}

func Actor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := strings.TrimSpace(r.Header.Get("X-Actor"))
		if actor == "" {
			actor = "api-user"
		}
		ctx := context.WithValue(r.Context(), actorKey{}, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type actorKey struct{}

func ActorFrom(ctx context.Context) string {
	actor, _ := ctx.Value(actorKey{}).(string)
	if actor == "" {
		return "api-user"
	}
	return actor
}

func remoteIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
