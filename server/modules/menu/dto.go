package menu

import "github.com/google/uuid"

// CreateMenuRequest represents the request body for creating a menu.
type CreateMenuRequest struct {
	Name      string         `json:"name" binding:"required"`
	Path      string         `json:"path" binding:"required"`
	Component string         `json:"component,omitempty"`
	Redirect  string         `json:"redirect,omitempty"`
	Icon      string         `json:"icon,omitempty"`
	ParentID  *uuid.UUID     `json:"parentId,omitempty"`
	Sort      int            `json:"sort"`
	Hidden    bool           `json:"hidden"`
	Affix     bool           `json:"affix"`
	Meta      map[string]any `json:"meta,omitempty"`
	Params    []string       `json:"params,omitempty"`
}

// BatchMenuInput represents a single menu in a batch create request.
// TempID and TempParentID enable parent-child references within the same batch.
type BatchMenuInput struct {
	TempID       string         `json:"tempId,omitempty"`
	TempParentID string         `json:"tempParentId,omitempty"`
	Name         string         `json:"name"`
	Path         string         `json:"path"`
	Component    string         `json:"component,omitempty"`
	Redirect     string         `json:"redirect,omitempty"`
	Icon         string         `json:"icon,omitempty"`
	ParentID     *uuid.UUID     `json:"parentId,omitempty"`
	Sort         int            `json:"sort"`
	Hidden       bool           `json:"hidden"`
	Affix        bool           `json:"affix"`
	Meta         map[string]any `json:"meta,omitempty"`
	Params       []string       `json:"params,omitempty"`
}

// UpdateMenuRequest represents the request body for updating a menu.
type UpdateMenuRequest struct {
	Name      string         `json:"name,omitempty"`
	Path      string         `json:"path,omitempty"`
	Component string         `json:"component,omitempty"`
	Redirect  string         `json:"redirect,omitempty"`
	Icon      string         `json:"icon,omitempty"`
	ParentID  *uuid.UUID     `json:"parentId,omitempty"`
	Sort      int            `json:"sort,omitempty"`
	Hidden    bool           `json:"hidden,omitempty"`
	Affix     bool           `json:"affix,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Params    []string       `json:"params,omitempty"`
}
