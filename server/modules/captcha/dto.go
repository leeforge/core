package captcha

// VerifyRequest 验证请求
type VerifyRequest struct {
	ID     string `json:"id" validate:"required"`     // 验证码 ID
	Answer string `json:"answer" validate:"required"` // 用户答案
}
