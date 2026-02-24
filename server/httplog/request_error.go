package httplog

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"
)

// Error writes a structured error log enriched with request context.
func Error(logger logging.Logger, r *http.Request, msg string, err error, fields ...zap.Field) {
	if logger == nil {
		return
	}

	allFields := make([]zap.Field, 0, len(fields)+8)
	if err != nil {
		allFields = append(allFields,
			zap.Error(err),
			zap.Strings("error_chain", errorChain(err)),
		)
	}

	if r != nil {
		allFields = append(allFields,
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("route", routePattern(r)),
		)
		if r.URL.RawQuery != "" {
			allFields = append(allFields, zap.String("query", r.URL.RawQuery))
		}
		if traceID := traceIDFromContext(r); traceID != "" {
			allFields = append(allFields, zap.String("trace_id", traceID))
		}
	}

	allFields = append(allFields, fields...)
	logger.Error(msg, allFields...)
}

func routePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}

func traceIDFromContext(r *http.Request) string {
	if r == nil {
		return ""
	}

	if traceID := logging.GetTraceID(r.Context()); traceID != "" {
		return traceID
	}
	if traceID, ok := r.Context().Value("trace_id").(string); ok {
		return traceID
	}

	return ""
}

func errorChain(err error) []string {
	chain := make([]string, 0, 4)
	for curr := err; curr != nil; curr = errors.Unwrap(curr) {
		chain = append(chain, curr.Error())
	}
	return chain
}
