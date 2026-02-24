package operationlog

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/leeforge/core/core"
	"github.com/leeforge/framework/logging"
)

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int64
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

// Middleware 创建操作日志记录中间件
func Middleware(service *OperationLogService, logger logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 1. 读取请求体 (如果需要记录)
			// 注意：大文件上传等场景应跳过
			var reqBody []byte
			if r.Body != nil && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				reqBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			// 2. 包装 ResponseWriter
			rw := &responseWriter{ResponseWriter: w}

			// 3. 处理请求
			next.ServeHTTP(rw, r)

			// 4. 准备日志数据 (在主 goroutine 中提取 context 信息)
			latency := time.Since(start)
			method := r.Method
			path := r.URL.Path
			query := r.URL.RawQuery
			clientIP := r.RemoteAddr // 实际应用中建议使用 x-forwarded-for 解析工具
			userAgent := r.UserAgent()
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}

			// 提取用户ID
			var userID *uuid.UUID
			if id, ok := core.GetIdentity(r.Context()); ok {
				uid := id.UserID
				userID = &uid
			}

			// 提取所属域ID（用于日志查询隔离）
			var ownerDomainID *uuid.UUID
			if domainID, ok := core.GetDomainID(r.Context()); ok {
				if parsedDomainID, err := uuid.Parse(domainID); err == nil {
					ownerDomainID = &parsedDomainID
				}
			}

			// 提取 TraceID (假设使用了 Trace 中间件)
			traceID := w.Header().Get("X-Trace-ID")

			// 5. 异步记录日志
			go func() {
				// 避免记录日志接口本身的查询日志，防止递归爆炸
				if method == "GET" && strings.HasPrefix(path, "/api/v1/logs") {
					return
				}

				// 简单的 Body 处理
				bodyStr := string(reqBody)
				if len(bodyStr) > 2000 {
					bodyStr = bodyStr[:2000] + "...(truncated)"
				}

				// TODO: 在这里可以添加敏感字段过滤逻辑 (如 password)

				service.Create(&CreateLogInput{
					RequestID:     traceID,
					Method:        method,
					Path:          path,
					QueryParams:   query,
					Body:          bodyStr,
					Status:        status,
					Latency:       latency.Nanoseconds(),
					ClientIP:      clientIP,
					UserAgent:     userAgent,
					UserID:        userID,
					OwnerDomainID: ownerDomainID,
					ErrorMessage:  "", // 可以在 error handler 中通过 context 传递错误信息
				})
			}()
		})
	}
}
