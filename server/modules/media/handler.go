package media

import (
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/core/core"
	apperrors "github.com/leeforge/core/server/pkg/errors"
	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type MediaHandler struct {
	mediaService *MediaService
	logger       logging.Logger
}

func NewMediaHandler(mediaService *MediaService, logger logging.Logger) *MediaHandler {
	return &MediaHandler{
		mediaService: mediaService,
		logger:       logger,
	}
}

func handleMediaServiceError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	var appErr *apperrors.AppError
	if stdErrors.As(err, &appErr) {
		status := apperrors.GetHTTPStatus(appErr.Code)
		responder.CustomError(w, r, status, appErr.Code, appErr.Message, nil)
		return true
	}

	return false
}

// ListMedia returns hierarchical mixed media nodes (folders + files).
// @Summary List media library nodes
// @Description Return mixed folder/file nodes under a folder context. This is the primary media library browse endpoint.
// @Tags Media
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param parentId query string false "Parent folder ID (UUID)"
// @Param path query string false "Folder path fallback, used when parentId is absent"
// @Param include query string false "Node type filter: all|folders|files" Enums(all,folders,files) default(all)
// @Param _q query string false "Search keyword (folder/file name, alias)"
// @Param mimePrefix query string false "MIME prefix filter, e.g. image/"
// @Param provider query string false "Storage provider filter"
// @Param sort query string false "Sort field: name|createdAt|updatedAt|size" Enums(name,createdAt,updatedAt,size) default(createdAt)
// @Param order query string false "Sort order: asc|desc" Enums(asc,desc) default(desc)
// @Param foldersFirst query bool false "Whether folders are ordered before files" default(true)
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media [get]
func (h *MediaHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var parentID *uuid.UUID
	if parentIDStr := r.URL.Query().Get("parentId"); parentIDStr != "" {
		parsed, err := uuid.Parse(parentIDStr)
		if err != nil {
			responder.BadRequest(w, r, "Invalid parentId")
			return
		}
		parentID = &parsed
	}

	foldersFirst := true
	if raw := r.URL.Query().Get("foldersFirst"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			responder.BadRequest(w, r, "Invalid foldersFirst")
			return
		}
		foldersFirst = parsed
	}

	opts := ListMediaLibraryOptions{
		ParentID:     parentID,
		Path:         r.URL.Query().Get("path"),
		Include:      r.URL.Query().Get("include"),
		Search:       r.URL.Query().Get("_q"),
		MimePrefix:   r.URL.Query().Get("mimePrefix"),
		Provider:     r.URL.Query().Get("provider"),
		Page:         page,
		PageSize:     pageSize,
		Sort:         r.URL.Query().Get("sort"),
		Order:        r.URL.Query().Get("order"),
		FoldersFirst: foldersFirst,
	}

	nodes, total, err := h.mediaService.ListMediaLibrary(r.Context(), opts)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to list media")
		return
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	pagination := &responder.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      int64(total),
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	responder.WriteList(w, r, http.StatusOK, nodes, pagination)
}

// Upload handles file upload
// @Summary Upload a file
// @Tags Media
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Param folder_path formData string false "Folder path" default("/")
// @Success 201 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media [post]
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		responder.BadRequest(w, r, "File too large or invalid format")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		responder.BadRequest(w, r, "No file provided")
		return
	}
	defer file.Close()

	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "User not found in context")
		return
	}

	folderPath := r.FormValue("folder_path")
	if folderPath == "" {
		folderPath = "/"
	}

	m, err := h.mediaService.UploadFile(r.Context(), header, userID, folderPath)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to upload file")
		return
	}

	responder.Created(w, r, m)
}

// GetMedia returns media details
// @Summary Get media details
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/{id} [get]
func (h *MediaHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid media ID")
		return
	}

	m, err := h.mediaService.GetMedia(r.Context(), id)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to get media")
		return
	}

	responder.OK(w, r, m)
}

// DeleteMedia deletes a media file
// @Summary Delete a media file
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/{id} [delete]
func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid media ID")
		return
	}

	if err := h.mediaService.DeleteMedia(r.Context(), id); err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to delete media")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Media deleted successfully"})
}

// UpdateMedia updates media metadata
// @Summary Update media metadata
// @Tags Media
// @Accept json,multipart/form-data
// @Produce json
// @Param id path string true "Media ID"
// @Param request body object true "Media update data"
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/{id} [put]
func (h *MediaHandler) UpdateMedia(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid media ID")
		return
	}

	var req UpdateMediaInput
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			responder.BadRequest(w, r, "Invalid multipart form data")
			return
		}
		req = UpdateMediaInput{
			Title:      r.FormValue("title"),
			AltText:    r.FormValue("altText"),
			Caption:    r.FormValue("caption"),
			FolderPath: r.FormValue("folderPath"),
		}

		thumbnailFile, thumbnailHeader, fileErr := r.FormFile("thumbnail")
		if fileErr == nil {
			req.Thumbnail = thumbnailHeader
			_ = thumbnailFile.Close()
		} else if !stdErrors.Is(fileErr, http.ErrMissingFile) {
			responder.BadRequest(w, r, "Invalid thumbnail file")
			return
		}
	} else {
		var jsonReq struct {
			Title      string `json:"title"`
			AltText    string `json:"altText"`
			Caption    string `json:"caption"`
			FolderPath string `json:"folderPath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&jsonReq); err != nil {
			responder.BadRequest(w, r, "Invalid request body")
			return
		}
		req = UpdateMediaInput{
			Title:      jsonReq.Title,
			AltText:    jsonReq.AltText,
			Caption:    jsonReq.Caption,
			FolderPath: jsonReq.FolderPath,
		}
	}

	m, err := h.mediaService.UpdateMediaWithInput(r.Context(), id, req)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to update media")
		return
	}

	responder.OK(w, r, m)
}

// GetSignedURL generates a temporary access URL
// @Summary Generate signed URL for private media
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Media ID"
// @Param expiry query int false "URL expiry time in seconds" default(3600)
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/{id}/signed [get]
func (h *MediaHandler) GetSignedURL(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid media ID")
		return
	}

	// Default expiry: 1 hour
	expiry := time.Hour
	if expiryStr := r.URL.Query().Get("expiry"); expiryStr != "" {
		if seconds, err := strconv.Atoi(expiryStr); err == nil && seconds > 0 {
			expiry = time.Duration(seconds) * time.Second
		}
	}

	url, err := h.mediaService.GetSignedURL(r.Context(), id, expiry)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to generate signed URL")
		return
	}

	responder.OK(w, r, map[string]string{"url": url})
}

// CreateFolder creates a media folder.
// @Summary Create media folder
// @Tags Media
// @Accept json
// @Produce json
// @Param request body media.CreateFolderInput true "Create folder payload"
// @Success 201 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 409 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/folders [post]
func (h *MediaHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req CreateFolderInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	folder, err := h.mediaService.CreateFolder(r.Context(), req)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to create folder")
		return
	}

	responder.Created(w, r, folder)
}

// UpdateFolder renames or moves a media folder.
// @Summary Update media folder
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Folder ID"
// @Param request body media.UpdateFolderInput true "Update folder payload"
// @Success 200 {object} responder.Response
// @Failure 400 {object} responder.Error
// @Failure 404 {object} responder.Error
// @Failure 409 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/folders/{id} [put]
func (h *MediaHandler) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid folder ID")
		return
	}

	var req UpdateFolderInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	folder, err := h.mediaService.UpdateFolder(r.Context(), id, req)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to update folder")
		return
	}

	responder.OK(w, r, folder)
}

// DeleteFolder deletes a media folder.
// @Summary Delete media folder
// @Description Non-empty folder deletion requires recursive=true.
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Folder ID"
// @Param recursive query bool false "Recursive delete" default(false)
// @Success 200 {object} media.FolderDeleteResult
// @Failure 400 {object} responder.Error
// @Failure 404 {object} responder.Error
// @Failure 409 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/folders/{id} [delete]
func (h *MediaHandler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid folder ID")
		return
	}

	recursive := false
	if raw := r.URL.Query().Get("recursive"); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			responder.BadRequest(w, r, "Invalid recursive flag")
			return
		}
		recursive = parsed
	}

	result, err := h.mediaService.DeleteFolder(r.Context(), id, recursive)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to delete folder")
		return
	}

	responder.OK(w, r, result)
}

// BulkMove moves files/folders to a destination folder.
// @Summary Bulk move media resources
// @Tags Media
// @Accept json
// @Produce json
// @Param request body media.BulkMoveInput true "Bulk move payload"
// @Success 200 {object} media.BulkMoveResult
// @Failure 400 {object} responder.Error
// @Failure 404 {object} responder.Error
// @Failure 409 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/actions/bulk-move [post]
func (h *MediaHandler) BulkMove(w http.ResponseWriter, r *http.Request) {
	var req BulkMoveInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	result, err := h.mediaService.BulkMove(r.Context(), req)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to bulk move")
		return
	}

	responder.OK(w, r, result)
}

// BulkDelete deletes files/folders in one request.
// @Summary Bulk delete media resources
// @Tags Media
// @Accept json
// @Produce json
// @Param request body media.BulkDeleteInput true "Bulk delete payload"
// @Success 200 {object} media.BulkDeleteResult
// @Failure 400 {object} responder.Error
// @Failure 404 {object} responder.Error
// @Failure 409 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /media/actions/bulk-delete [post]
func (h *MediaHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var req BulkDeleteInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	result, err := h.mediaService.BulkDelete(r.Context(), req)
	if err != nil {
		if handleMediaServiceError(w, r, err) {
			return
		}
		responder.InternalServerError(w, r, "Failed to bulk delete")
		return
	}

	responder.OK(w, r, result)
}

// SetupMediaRoutes registers media routes
func SetupMediaRoutes(r chi.Router, h *MediaHandler, jwtMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/media", func(r chi.Router) {
		r.Use(jwtMiddleware)

		r.Get("/", h.ListMedia)
		r.Post("/", h.Upload)
		r.Post("/folders", h.CreateFolder)
		r.Put("/folders/{id}", h.UpdateFolder)
		r.Delete("/folders/{id}", h.DeleteFolder)
		r.Post("/actions/bulk-move", h.BulkMove)
		r.Post("/actions/bulk-delete", h.BulkDelete)
		r.Get("/{id}", h.GetMedia)
		r.Delete("/{id}", h.DeleteMedia)
		r.Put("/{id}", h.UpdateMedia)
		r.Get("/{id}/signed", h.GetSignedURL)
	})
}
