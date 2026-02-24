package captcha

import (
	"context"
	"fmt"
	"time"

	frameCaptcha "github.com/leeforge/framework/captcha"

	"github.com/mojocn/base64Captcha"
)

// MathGenerator 数学验证码生成器
type MathGenerator struct {
	config frameCaptcha.MathConfig
}

// NewMathGenerator 创建数学验证码生成器
func NewMathGenerator(config frameCaptcha.MathConfig) frameCaptcha.Generator {
	return &MathGenerator{config: config}
}

func (g *MathGenerator) Generate(ctx context.Context, captchaType frameCaptcha.CaptchaType) (*frameCaptcha.CaptchaData, string, error) {
	if captchaType != frameCaptcha.TypeMath {
		return nil, "", fmt.Errorf("不支持的类型: %s", captchaType)
	}

	driver := base64Captcha.DriverMath{
		Width:           g.config.Width,
		Height:          g.config.Height,
		NoiseCount:      g.config.NoiseCount,
		ShowLineOptions: g.config.ShowLineOptions,
	}

	// 仅用于生成的临时内存存储
	tempStore := base64Captcha.DefaultMemStore
	captcha := base64Captcha.NewCaptcha(driver.ConvertFonts(), tempStore)

	id, b64s, answer, err := captcha.Generate()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", frameCaptcha.ErrGenerationFailed, err)
	}

	data := &frameCaptcha.CaptchaData{
		ID:        id,
		Type:      frameCaptcha.TypeMath,
		Content:   b64s,
		ExpiresAt: time.Now().Add(5 * time.Minute), // 会被 Store TTL 覆盖
	}

	return data, answer, nil
}
