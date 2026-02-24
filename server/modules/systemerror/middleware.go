package systemerror

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

// RecoveryMiddleware captures panics and records them
func RecoveryMiddleware(service *SystemErrorService, logger logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 1. Get stack trace
					stack := debug.Stack()
					errorMsg := fmt.Sprintf("%v", err)

					logger.Error("Panic recovered",
						zap.String("error", errorMsg),
						zap.String("stack", string(stack)),
					)

					// 2. Extract context info
					var userID *uuid.UUID
					if id, ok := core.GetIdentity(r.Context()); ok {
						uid := id.UserID
						userID = &uid
					}
					traceID := w.Header().Get("X-Trace-ID")

					// 3. Record to DB
					service.Create(&CreateErrorInput{
						ErrorLevel:    "critical",
						ErrorType:     "panic",
						ErrorMsg:      errorMsg,
						ErrorStack:    string(stack),
						RequestID:     traceID,
						RequestMethod: r.Method,
						RequestPath:   r.URL.Path,
						UserID:        userID,
					})

					// 4. Return 500 response
					responder.InternalServerError(w, r, "Internal Server Error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
