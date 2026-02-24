package echoadapter

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"

	corebootstrap "github.com/leeforge/core/bootstrap"
	"github.com/leeforge/core/host"
)

func RegisterAllEcho(e *echo.Echo, opts host.CoreOptions) error {
	if e == nil {
		return fmt.Errorf("echo instance is nil")
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

	forward := echo.WrapHandler(chiRouter)
	apiPrefix := resolveGatewayPrefix(opts.BasePath)
	e.Any(apiPrefix+"/*", forward)
	e.GET("/swagger", forward)
	e.Any("/swagger/*", forward)
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
