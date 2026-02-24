package domain

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type DomainHandler struct {
	domainService core.DomainResolver
	logger        logging.Logger
}

func NewDomainHandler(domainService core.DomainResolver, logger logging.Logger) *DomainHandler {
	return &DomainHandler{
		domainService: domainService,
		logger:        logger,
	}
}

// ListMyDomains godoc
// @Summary      List current user's domains
// @Description  Returns all domains the authenticated user belongs to
// @Tags         Domains
// @Produce      json
// @Success      200  {array}  core.UserDomainInfo
// @Failure      401  {object} map[string]string
// @Router       /domains/me [get]
func (h *DomainHandler) ListMyDomains(w http.ResponseWriter, r *http.Request) {
	identity, ok := core.GetIdentity(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "Missing identity")
		return
	}

	domains, err := h.domainService.ListUserDomains(r.Context(), identity.UserID)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list user domains", err, zap.String("user_id", identity.UserID.String()))
		responder.InternalServerError(w, r, "Failed to list domains")
		return
	}

	responder.OK(w, r, domains)
}
