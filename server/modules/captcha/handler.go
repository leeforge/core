package captcha

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/leeforge/framework/http/responder"
)

// Handler HTTP 处理器
type Handler struct {
	service frameCaptcha.Service
	config  *frameCaptcha.Config
}

// NewHandler 创建处理器
func NewHandler(service frameCaptcha.Service, config *frameCaptcha.Config) *Handler {
	return &Handler{
		service: service,
		config:  config,
	}
}

// Generate godoc
// @Summary 生成验证码
// @Description 生成新的验证码挑战
// @Tags captcha
// @Accept json
// @Produce json
// @Param type query string false "验证码类型 (math|image)" default(math)
// @Success 200 {object} responder.Response
// @Failure 429 {object} responder.Response
// @Router /captcha [get]
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	captchaType := frameCaptcha.CaptchaType(r.URL.Query().Get("type"))
	if captchaType == "" {
		captchaType = frameCaptcha.TypeMath
	}

	// 使用 IP 作为标识符（也可以使用 session ID）
	identifier := clientIdentifier(r.RemoteAddr)

	data, err := h.service.Generate(r.Context(), captchaType, identifier)
	if err != nil {
		if err == frameCaptcha.ErrRateLimitExceeded {
			responder.TooManyRequests(w, r, "生成验证码过于频繁")
			return
		}
		responder.InternalServerError(w, r, "生成验证码失败")
		return
	}

	responder.OK(w, r, data)
}

// Verify godoc
// @Summary 验证验证码
// @Description 验证验证码答案
// @Tags captcha
// @Accept json
// @Produce json
// @Param request body VerifyRequest true "验证请求"
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Response
// @Failure 429 {object} responder.Response
// @Router /captcha/verify [post]
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "请求参数错误")
		return
	}

	identifier := clientIdentifier(r.RemoteAddr)

	result, err := h.service.Verify(r.Context(), req.ID, req.Answer, identifier)
	if err != nil {
		if err == frameCaptcha.ErrRateLimitExceeded {
			failureReason := "尝试次数过多"
			if result != nil && strings.TrimSpace(result.FailureReason) != "" {
				failureReason = result.FailureReason
			}
			responder.TooManyRequests(w, r, failureReason)
			return
		}
		responder.InternalServerError(w, r, "验证验证码失败")
		return
	}

	responder.OK(w, r, result)
}

func clientIdentifier(remoteAddr string) string {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return trimmed
	}

	return host
}
