package middleware

import (
	"net/http"

	"github.com/leeforge/framework/http/responder"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"

	"github.com/leeforge/core/core"
)

// DomainMembershipConfig configures domain membership validation.
type DomainMembershipConfig struct {
	Logger        logging.Logger
	DomainService core.DomainResolver
}

// DomainMembershipMiddleware validates that the user is a member of the acting domain.
// Skips for platform:root domain with super_admin, and when DomainService is nil.
func DomainMembershipMiddleware(cfg *DomainMembershipConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg == nil || cfg.DomainService == nil {
				next.ServeHTTP(w, r)
				return
			}

			identity, ok := core.GetIdentity(r.Context())
			if !ok {
				responder.Unauthorized(w, r, "Missing identity")
				return
			}

			ac := core.GetActingContext(r.Context())
			if ac == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Skip for platform domain + super admin
			if ac.IsPlatformDomain() && identity.IsSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if no resolved domain
			if ac.Domain == nil {
				next.ServeHTTP(w, r)
				return
			}

			isMember, err := cfg.DomainService.CheckMembership(
				r.Context(),
				ac.Domain.DomainID,
				identity.UserID,
			)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Error("Failed to check domain membership",
						zap.Error(err),
						zap.String("user_id", identity.UserID.String()),
						zap.String("domain_id", ac.Domain.DomainID.String()),
					)
				}
				responder.InternalServerError(w, r, "Failed to verify domain membership")
				return
			}

			if !isMember {
				responder.Forbidden(w, r, "Domain access denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
