package errors

import "fmt"

// Error codes for the application
// 4xxx - Client errors
// 5xxx - Server errors

const (
	// 4xxx 客户端错误
	ErrCodeBindFailed        = 4001 // 参数绑定错误
	ErrCodeValidationFailed  = 4002 // 数据验证失败
	ErrCodeNotFound          = 4003 // 资源不存在
	ErrCodeRouteNotFound     = 4004 // 路由不存在
	ErrCodeForbidden         = 4005 // 权限不足
	ErrCodeUnauthorized      = 4006 // 认证失败
	ErrCodeDuplicate         = 4007 // 资源已存在
	ErrCodeRateLimitExceeded = 4008 // 速率限制

	// Initialization errors
	ErrSystemAlreadyInitialized = 4009 // 系统已初始化
	ErrInvalidInitKey           = 4010 // 无效的初始化密钥
	ErrInitDataCreationFailed   = 5005 // 初始化数据创建失败

	// 5xxx 服务端错误
	ErrCodeDatabase       = 5001 // 数据库错误
	ErrCodeBusinessLogic  = 5002 // 业务逻辑错误
	ErrCodeFileUpload     = 5003 // 文件上传失败
	ErrCodeStorageService = 5004 // 存储服务错误
	ErrCodeInternal       = 5000 // 内部错误
)

// ErrorMessages maps error codes to default messages
var ErrorMessages = map[int]string{
	ErrCodeBindFailed:           "参数绑定错误",
	ErrCodeValidationFailed:     "数据验证失败",
	ErrCodeNotFound:             "资源不存在",
	ErrCodeRouteNotFound:        "路由不存在",
	ErrCodeForbidden:            "权限不足",
	ErrCodeUnauthorized:         "认证失败",
	ErrCodeDuplicate:            "资源已存在",
	ErrCodeRateLimitExceeded:    "速率限制超出",
	ErrSystemAlreadyInitialized: "系统已初始化",
	ErrInvalidInitKey:           "无效的初始化密钥",
	ErrInitDataCreationFailed:   "初始化数据创建失败",
	ErrCodeDatabase:             "数据库错误",
	ErrCodeBusinessLogic:        "业务逻辑错误",
	ErrCodeFileUpload:           "文件上传失败",
	ErrCodeStorageService:       "存储服务错误",
	ErrCodeInternal:             "内部错误",
}

// GetMessage returns the default message for an error code
func GetMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// GetHTTPStatus maps error codes to HTTP status codes
func GetHTTPStatus(code int) int {
	switch {
	case code >= 4000 && code < 4100:
		// Client errors map to 400-499
		switch code {
		case ErrCodeNotFound, ErrCodeRouteNotFound:
			return 404
		case ErrCodeUnauthorized:
			return 401
		case ErrCodeForbidden:
			return 403
		case ErrCodeDuplicate:
			return 409
		case ErrCodeRateLimitExceeded:
			return 429
		default:
			return 400
		}
	case code >= 5000 && code < 6000:
		// Server errors map to 500-599
		return 500
	default:
		return 500
	}
}

// AppError represents an application error with code and message
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeNotFound,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewConflictError creates a new conflict error
func NewConflictError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeDuplicate,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewInternalError creates a new internal error
func NewInternalError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeInternal,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeForbidden,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewValidationError creates a new validation error
func NewValidationError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeValidationFailed,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}

// NewBadRequestError creates a new bad request error
func NewBadRequestError(message string, err ...error) *AppError {
	appErr := &AppError{
		Code:    ErrCodeBindFailed,
		Message: message,
	}
	if len(err) > 0 && err[0] != nil {
		appErr.Err = err[0]
	}
	return appErr
}
