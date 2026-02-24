package media

import (
	"time"

	"github.com/google/uuid"
)

// ListMediaLibraryOptions defines filters for hierarchical media browsing.
type ListMediaLibraryOptions struct {
	ParentID     *uuid.UUID
	Path         string
	Include      string
	Search       string
	MimePrefix   string
	Provider     string
	Page         int
	PageSize     int
	Sort         string
	Order        string
	FoldersFirst bool
}

// MediaLibraryNode is a mixed folder/file node for GET /media listing.
type MediaLibraryNode struct {
	ID        uuid.UUID  `json:"id"`
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	ParentID  *uuid.UUID `json:"parentId,omitempty"`
	Path      string     `json:"path"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`

	ChildrenCount *int   `json:"childrenCount,omitempty"`
	FilesCount    *int   `json:"filesCount,omitempty"`
	SizeBytes     *int64 `json:"sizeBytes,omitempty"`

	Mime     *string  `json:"mime,omitempty"`
	Size     *float64 `json:"size,omitempty"`
	URL      *string  `json:"url,omitempty"`
	Width    *int     `json:"width,omitempty"`
	Height   *int     `json:"height,omitempty"`
	Alias    *string  `json:"alias,omitempty"`
	Provider *string  `json:"provider,omitempty"`
}

type CreateFolderInput struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	ParentID *uuid.UUID `json:"parentId"`
}

type UpdateFolderInput struct {
	Name       *string    `json:"name,omitempty"`
	ParentID   *uuid.UUID `json:"parentId,omitempty"`
	MoveToRoot bool       `json:"moveToRoot,omitempty"`
}

type FolderDeleteResult struct {
	DeletedFolders int `json:"deletedFolders"`
	DeletedFiles   int `json:"deletedFiles"`
}

type BulkMoveInput struct {
	FileIDs             []uuid.UUID `json:"fileIds"`
	FolderIDs           []uuid.UUID `json:"folderIds"`
	DestinationFolderID *uuid.UUID  `json:"destinationFolderId"`
	DestinationRoot     bool        `json:"destinationRoot,omitempty"`
}

type BulkMoveResult struct {
	MovedFiles   int `json:"movedFiles"`
	MovedFolders int `json:"movedFolders"`
}

type BulkDeleteInput struct {
	FileIDs   []uuid.UUID `json:"fileIds"`
	FolderIDs []uuid.UUID `json:"folderIds"`
	Recursive bool        `json:"recursive"`
}

type BulkDeleteResult struct {
	DeletedFiles   int `json:"deletedFiles"`
	DeletedFolders int `json:"deletedFolders"`
}
