package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/pkg/jwt"

	"github.com/leeforge/framework/http/responder"
)

// JWTMiddleware verifies the JWT token and injects Identity into context
func JWTMiddleware(jwtService *jwt.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				responder.Unauthorized(w, r, "Missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				responder.BadRequest(w, r, "Invalid authorization format")
				return
			}

			tokenString := parts[1]

			// 2. Blacklist check
			if jwtService.IsTokenBlacklisted(tokenString) {
				responder.Unauthorized(w, r, "Token has been invalidated")
				return
			}

			// 3. Validate
			claims, err := jwtService.ValidateAccessToken(tokenString)
			if err != nil {
				responder.Unauthorized(w, r, "Invalid or expired token")
				return
			}

			// 4. Create Unified Identity
			ctxTenantID, _ := core.GetTenantID(r.Context())
			if claims.TenantID != "" && ctxTenantID != "" && claims.TenantID != ctxTenantID {
				if !claims.IsSuperAdmin {
					responder.Forbidden(w, r, "Tenant mismatch")
					return
				}
			}

			tenantID := claims.TenantID
			if tenantID == "" {
				tenantID = ctxTenantID
			} else if ctxTenantID != "" && claims.TenantID != ctxTenantID && claims.IsSuperAdmin {
				tenantID = ctxTenantID
			}
			identity := core.Identity{
				UserID:   claims.UserID,
				Username: claims.Username,
				Type:     core.IdentityTypeJWT,
				Roles:    claims.Roles,
				TenantID: tenantID,
				// Cross-tenant bypass is an explicit identity attribute, not role-name inferred.
				IsSuperAdmin: claims.IsSuperAdmin,
				// Permissions could be loaded here or by a separate RBAC middleware
			}

			// 5. Inject into context
			ctx := core.WithIdentity(r.Context(), identity)
			if tenantID != "" {
				ctx = core.WithTenantID(ctx, tenantID)
			}

			if claims.IsSuperAdmin {
				ctx = context.WithValue(ctx, "is_super_admin", true)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
