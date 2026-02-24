package dictionary

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/core/server/ent"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type DictionaryHandler struct {
	dictService *DictionaryService
	logger      logging.Logger
}

func NewDictionaryHandler(dictService *DictionaryService, logger logging.Logger) *DictionaryHandler {
	return &DictionaryHandler{
		dictService: dictService,
		logger:      logger,
	}
}

// ListDictionaries returns all dictionaries
// @Summary List all dictionaries
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Success 200 {object} []ent.Dictionary
// @Failure 500 {object} responder.Error
// @Router /dictionaries [get]
func (h *DictionaryHandler) ListDictionaries(w http.ResponseWriter, r *http.Request) {
	dicts, err := h.dictService.ListDictionaries(r.Context())
	if err != nil {
		responder.InternalServerError(w, r, "Failed to list dictionaries")
		return
	}
	responder.OK(w, r, dicts)
}

// CreateDictionary creates a new dictionary category
// @Summary Create a new dictionary
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param request body ent.Dictionary true "Dictionary data"
// @Success 201 {object} ent.Dictionary
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionaries [post]
func (h *DictionaryHandler) CreateDictionary(w http.ResponseWriter, r *http.Request) {
	var input ent.Dictionary
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	d, err := h.dictService.CreateDictionary(r.Context(), &input)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to create dictionary")
		return
	}
	responder.Created(w, r, d)
}

// UpdateDictionary updates a dictionary category
// @Summary Update a dictionary
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param id path string true "Dictionary ID"
// @Param request body ent.Dictionary true "Dictionary data"
// @Success 200 {object} ent.Dictionary
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionaries/{id} [put]
func (h *DictionaryHandler) UpdateDictionary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid dictionary ID")
		return
	}

	var input ent.Dictionary
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	d, err := h.dictService.UpdateDictionary(r.Context(), id, &input)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to update dictionary")
		return
	}
	responder.OK(w, r, d)
}

// DeleteDictionary deletes a dictionary category
// @Summary Delete a dictionary
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param id path string true "Dictionary ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionaries/{id} [delete]
func (h *DictionaryHandler) DeleteDictionary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid dictionary ID")
		return
	}

	if err := h.dictService.DeleteDictionary(r.Context(), id); err != nil {
		responder.InternalServerError(w, r, "Failed to delete dictionary")
		return
	}
	responder.OK(w, r, map[string]string{"message": "Dictionary deleted successfully"})
}

// CreateDetail creates a new dictionary item
// @Summary Create a dictionary detail item
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param id path string true "Dictionary ID"
// @Param request body ent.DictionaryDetail true "Dictionary detail data"
// @Success 201 {object} ent.DictionaryDetail
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionaries/{id}/details [post]
func (h *DictionaryHandler) CreateDetail(w http.ResponseWriter, r *http.Request) {
	dictIDStr := chi.URLParam(r, "id")
	dictID, err := uuid.Parse(dictIDStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid dictionary ID")
		return
	}

	var input ent.DictionaryDetail
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	item, err := h.dictService.CreateDetail(r.Context(), dictID, &input)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to create dictionary item")
		return
	}
	responder.Created(w, r, item)
}

// UpdateDetail updates a dictionary item
// @Summary Update a dictionary detail item
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param id path string true "Dictionary Detail ID"
// @Param request body ent.DictionaryDetail true "Dictionary detail data"
// @Success 200 {object} ent.DictionaryDetail
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionary-details/{id} [put]
func (h *DictionaryHandler) UpdateDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid item ID")
		return
	}

	var input ent.DictionaryDetail
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	item, err := h.dictService.UpdateDetail(r.Context(), id, &input)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to update dictionary item")
		return
	}
	responder.OK(w, r, item)
}

// DeleteDetail deletes a dictionary item
// @Summary Delete a dictionary detail item
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param id path string true "Dictionary Detail ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionary-details/{id} [delete]
func (h *DictionaryHandler) DeleteDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		responder.BadRequest(w, r, "Invalid item ID")
		return
	}

	if err := h.dictService.DeleteDetail(r.Context(), id); err != nil {
		responder.InternalServerError(w, r, "Failed to delete dictionary item")
		return
	}
	responder.OK(w, r, map[string]string{"message": "Dictionary item deleted successfully"})
}

// GetDictionaryByCode returns dictionary items by code (Public API)
// @Summary Get dictionary by code
// @Tags Dictionaries
// @Accept json
// @Produce json
// @Param code path string true "Dictionary code"
// @Success 200 {object} []ent.DictionaryDetail
// @Failure 400 {object} responder.Error
// @Failure 500 {object} responder.Error
// @Router /dictionaries/{code}/details [get]
func (h *DictionaryHandler) GetDictionaryByCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		responder.BadRequest(w, r, "Dictionary code required")
		return
	}

	items, err := h.dictService.GetDetailsByCode(r.Context(), code)
	if err != nil {
		responder.InternalServerError(w, r, "Failed to get dictionary items")
		return
	}

	responder.OK(w, r, items)
}
