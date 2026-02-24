package ginadapter

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	corebootstrap "github.com/leeforge/core/bootstrap"

	"github.com/leeforge/core/host"
)

func RegisterAllGin(engine *gin.Engine, opts host.CoreOptions) error {
	if engine == nil {
		return fmt.Errorf("gin engine is nil")
	}
	chiRouter, err := corebootstrap.NewChiRouter(&opts)
	if err != nil {
		return err
	}
	if err := host.RegisterSwaggerChi(chiRouter, &host.SwaggerOptions{
		BasePath: opts.BasePath,
	}); err != nil {
		return err
	}

	forward := gin.WrapH(chiRouter)
	apiPrefix := resolveGatewayPrefix(opts.BasePath)
	engine.Any(apiPrefix+"/*any", forward)
	engine.GET("/swagger", forward)
	engine.Any("/swagger/*any", forward)
	return nil
}

func resolveGatewayPrefix(basePath string) string {
	clean := strings.TrimSpace(basePath)
	if clean == "" {
		clean = host.DefaultBasePath
	}
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		return "/api"
	}
	parts := strings.Split(clean, "/")
	return "/" + parts[0]
}
