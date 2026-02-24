package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/leeforge/framework/logging"
	"go.uber.org/zap"
)

// TraceIDMiddleware adds a trace ID to each request
// It retrieves the trace ID from the X-Trace-ID header, or generates a new one if not present
func TraceIDMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), "trace_id", traceID)
			ctx = context.WithValue(ctx, "start_time", time.Now())

			w.Header().Set("X-Trace-ID", traceID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RecoveryMiddleware recovers from panics and logs them using provided logger.
func RecoveryMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Recovered from panic",
						zap.Any("error", err),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("trace_id", getTraceID(r.Context())),
					)

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs incoming requests and outgoing responses using provided logger.
func LoggingMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			logger.Info("Incoming request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("trace_id", getTraceID(r.Context())),
			)

			rw := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			logger.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.statusCode),
				zap.Duration("duration", duration),
				zap.String("trace_id", getTraceID(r.Context())),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return traceID
	}
	return ""
}

// LoggingConfig holds configuration for the colored logging middleware.
type LoggingConfig struct {
	// TimeFormat is the format for timestamps (default: "2006/01/02 - 15:04:05").
	TimeFormat string
	// Prefix is the log line prefix (default: "[HTTP]").
	Prefix string
	// SkipPaths is a list of paths to skip logging for (e.g., health checks).
	SkipPaths []string
	// ColorScheme is the color scheme to use (default: NewBackgroundColorScheme()).
	ColorScheme logging.ColorScheme
	// DisableColors disables colored output. Default false (colors enabled).
	DisableColors bool
}

// DefaultLoggingConfig returns a LoggingConfig with sensible defaults.
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		TimeFormat:    "2006/01/02 - 15:04:05",
		Prefix:        "[HTTP]",
		SkipPaths:     nil,
		ColorScheme:   logging.NewBackgroundColorScheme(),
		DisableColors: false,
	}
}

// ColoredLoggingMiddleware logs HTTP requests with colored terminal output.
// Output format:
// [HTTP] 2024/01/15 - 10:30:45 | 200 |      1.23ms |    127.0.0.1 | GET     /api/v1/users
func ColoredLoggingMiddleware(cfg *LoggingConfig) func(next http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultLoggingConfig()
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "2006/01/02 - 15:04:05"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "[HTTP]"
	}
	if cfg.ColorScheme == nil {
		cfg.ColorScheme = logging.NewBackgroundColorScheme()
	}

	skipPaths := make(map[string]bool)
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for configured paths
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start)

			// Build and print log line
			if cfg.DisableColors {
				printPlainLog(cfg, r, rw.statusCode, duration, start)
			} else {
				printColoredLog(cfg, r, rw.statusCode, duration, start)
			}
		})
	}
}

func printColoredLog(cfg *LoggingConfig, r *http.Request, status int, duration time.Duration, start time.Time) {
	scheme := cfg.ColorScheme

	// Format each part with colors
	timestamp := logging.Colorize(logging.Gray, start.Format(cfg.TimeFormat))
	statusStr := logging.Styled(fmt.Sprintf(" %d ", status), scheme.StatusColor(status))
	durationStr := logging.Colorizef(scheme.DurationColor(duration), "%12s", duration.Round(time.Microsecond))
	clientIP := logging.Colorizef(logging.Gray, "%15s", getClientIP(r))
	method := logging.Colorizef(scheme.MethodColor(r.Method), "%-7s", r.Method)
	path := r.URL.Path

	fmt.Printf("%s %s |%s|%s | %s | %s %s\n",
		cfg.Prefix,
		timestamp,
		statusStr,
		durationStr,
		clientIP,
		method,
		path,
	)
}

func printPlainLog(cfg *LoggingConfig, r *http.Request, status int, duration time.Duration, start time.Time) {
	fmt.Printf("%s %s | %3d | %12s | %15s | %-7s %s\n",
		cfg.Prefix,
		start.Format(cfg.TimeFormat),
		status,
		duration.Round(time.Microsecond),
		getClientIP(r),
		r.Method,
		r.URL.Path,
	)
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
