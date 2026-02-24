package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/leeforge/framework/http/responder"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"

	"github.com/leeforge/core/core"
)

const (
	metadataActingTenantID    = "acting_tenant_id"
	metadataIsImpersonating   = "is_impersonating"
	metadataImpersonateReason = "impersonate_reason"
	metadataImpersonateExpiry = "impersonate_expiry"
)

// DomainResolverConfig configures the domain resolver middleware.
type DomainResolverConfig struct {
	Logger        logging.Logger
	DomainService core.DomainResolver
}

// DomainResolverMiddleware resolves acting domain with protocol support.
//
// Resolution priority:
//  1. X-Domain-Type + X-Domain-Key headers (new protocol)
//  2. X-Tenant-ID header (backward compat, maps to tenant:<value>)
//  3. Token defaultDomain from DomainMembership (is_default=true)
//  4. Platform fallback for super admins
func DomainResolverMiddleware(cfg *DomainResolverConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := core.GetIdentity(r.Context())
			if !ok {
				responder.Unauthorized(w, r, "Missing identity")
				return
			}

			ac := &core.ActingContext{
				ActorID: identity.UserID,
			}

			resolved := false

			// Priority 1: New protocol headers
			domainType := strings.TrimSpace(r.Header.Get("X-Domain-Type"))
			domainKey := strings.TrimSpace(r.Header.Get("X-Domain-Key"))
			if domainType != "" && domainKey != "" && cfg.DomainService != nil {
				rd, err := cfg.DomainService.ResolveDomain(r.Context(), domainType, domainKey)
				if err != nil {
					if cfg.Logger != nil {
						cfg.Logger.Warn("Failed to resolve domain from headers",
							zap.String("type", domainType),
							zap.String("key", domainKey),
							zap.Error(err),
						)
					}
					responder.BadRequest(w, r, "Invalid domain")
					return
				}
				ac.Domain = rd
				resolved = true

				// Backward compat: if tenant type, also set identity tenant context
				if domainType == "tenant" {
					identity.TenantID = domainKey
				}
			}

			// Priority 2: Legacy X-Tenant-ID header
			if !resolved {
				headerTenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
				if headerTenantID != "" {
					identity.TenantID = headerTenantID

					// Try to resolve from DB for the Domain field
					if cfg.DomainService != nil {
						if rd, err := cfg.DomainService.ResolveDomain(r.Context(), "tenant", headerTenantID); err == nil {
							ac.Domain = rd
						}
					}
					resolved = true
				}
			}

			// Priority 3: Default domain from membership
			if !resolved && cfg.DomainService != nil {
				rd, err := cfg.DomainService.GetUserDefaultDomain(r.Context(), identity.UserID)
				if err == nil && rd != nil {
					ac.Domain = rd

					if rd.TypeCode == "tenant" {
						identity.TenantID = rd.Key
					}
					resolved = true
				}
			}

			// Priority 4: Super admin fallback or legacy resolution
			if !resolved {
				if identity.IsSuperAdmin {
					ac.Domain = &core.ResolvedDomain{
						TypeCode: string(core.DomainPlatform),
						Key:      "root",
					}
					resolveImpersonation(&identity, ac)
				} else {
					if !resolveTenantDomain(&identity, ac) {
						responder.Forbidden(w, r, "Missing or invalid tenant domain")
						return
					}
				}
			}

			// Inject both old and new context values for backward compatibility
			ctx := core.WithIdentity(r.Context(), identity)
			ctx = core.WithTenantID(ctx, tenantIDFromActingContext(ac))
			ctx = core.WithActingContext(ctx, ac)
			if ac.Domain != nil {
				ctx = core.WithDomainID(ctx, ac.Domain.DomainID.String())
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveImpersonation(identity *core.Identity, ac *core.ActingContext) {
	if identity == nil || ac == nil {
		return
	}

	if applyImpersonationClaims(identity, ac) {
		return
	}

	ac.Domain = &core.ResolvedDomain{
		TypeCode: string(core.DomainPlatform),
		Key:      "root",
	}
	identity.TenantID = ""
}

func resolveTenantDomain(identity *core.Identity, ac *core.ActingContext) bool {
	if identity == nil || ac == nil {
		return false
	}

	if isImpersonating(identity) {
		return applyImpersonationClaims(identity, ac)
	}

	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		return false
	}
	ac.Domain = &core.ResolvedDomain{
		TypeCode: "tenant",
		Key:      tenantID,
	}
	return true
}

func isImpersonating(identity *core.Identity) bool {
	if identity == nil || identity.Metadata == nil {
		return false
	}
	imp, _ := identity.Metadata[metadataIsImpersonating].(bool)
	return imp
}

func applyImpersonationClaims(identity *core.Identity, ac *core.ActingContext) bool {
	if identity == nil || ac == nil || identity.Metadata == nil {
		return false
	}

	imp, _ := identity.Metadata[metadataIsImpersonating].(bool)
	if !imp {
		return false
	}

	tenantID, _ := identity.Metadata[metadataActingTenantID].(string)
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return false
	}

	if _, err := uuid.Parse(tenantID); err != nil {
		return false
	}

	if expiryUnix, ok := parseImpersonationExpiryUnix(identity.Metadata[metadataImpersonateExpiry]); ok && expiryUnix > 0 {
		expiry := time.Unix(expiryUnix, 0)
		if time.Now().After(expiry) {
			return false
		}
		ac.ImpersonateExpiry = expiry
	}

	if reason, ok := identity.Metadata[metadataImpersonateReason].(string); ok {
		ac.ImpersonateReason = reason
	}

	ac.IsImpersonating = true
	ac.Domain = &core.ResolvedDomain{
		TypeCode: "tenant",
		Key:      tenantID,
	}
	identity.TenantID = tenantID
	return true
}

func parseImpersonationExpiryUnix(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

func tenantIDFromActingContext(ac *core.ActingContext) string {
	if ac == nil || ac.Domain == nil {
		return ""
	}
	if ac.Domain.TypeCode == "tenant" {
		return ac.Domain.Key
	}
	return ""
}
