package initmod

import (
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/leeforge/core/server/httplog"
	"github.com/leeforge/core/server/pkg/errors"

	"github.com/leeforge/framework/http/binding"
	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type InitHandler struct {
	service *InitService
	logger  logging.Logger
}

func NewInitHandler(service *InitService, logger logging.Logger) *InitHandler {
	return &InitHandler{
		service: service,
		logger:  logger,
	}
}

// Status checks the system initialization status
// @Summary Check initialization status
// @Description Check if the system has been initialized
// @Tags Initialization
// @Accept json
// @Produce json
// @Success 200 {object} InitStatusResponse
// @Failure 500 {object} responder.Error
// @Router /init/status [get]
func (h *InitHandler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status, err := h.service.CheckStatus(ctx)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to check init status", err)
		responder.CustomError(w, r, http.StatusInternalServerError, errors.ErrCodeInternal, "Failed to check status", err)
		return
	}

	responder.OK(w, r, status)
}

// Setup performs the system initialization
// @Summary Initialize system
// @Description Initialize the system with platform super-admin account
// @Tags Initialization
// @Accept json
// @Produce json
// @Param request body InitSetupRequest true "Initialization data"
// @Success 200 {object} InitSetupResponse
// @Failure 400 {object} responder.Error
// @Failure 401 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /init/setup [post]
func (h *InitHandler) Setup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req InitSetupRequest
	// Use binding.JSON which handles reading, unmarshalling and validation
	if err := binding.JSON(r, &req); err != nil {
		responder.BindError(w, r, err)
		return
	}

	result, err := h.service.Initialize(ctx, &req)
	if err != nil {
		httplog.Error(h.logger, r, "Initialization failed", err)

		// Map errors to appropriate HTTP status codes
		errMsg := err.Error()
		if strings.Contains(errMsg, "system already initialized") {
			responder.CustomError(w, r, http.StatusConflict, errors.ErrSystemAlreadyInitialized, errors.GetMessage(errors.ErrSystemAlreadyInitialized), err)
		} else if strings.Contains(errMsg, "invalid initialization key") {
			responder.CustomError(w, r, http.StatusUnauthorized, errors.ErrInvalidInitKey, errors.GetMessage(errors.ErrInvalidInitKey), err)
		} else if strings.Contains(errMsg, "user already exists") {
			responder.CustomError(w, r, http.StatusBadRequest, errors.ErrCodeDuplicate, "Admin user already exists", err)
		} else {
			responder.CustomError(w, r, http.StatusInternalServerError, errors.ErrCodeInternal, "Initialization failed", err)
		}
		return
	}

	h.logger.Info("System initialized successfully",
		zap.String("admin_id", result.AdminUser.ID),
		zap.Int("roles", result.RolesCreated),
	)

	responder.OK(w, r, result)
}
