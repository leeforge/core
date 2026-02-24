package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/media"
	"github.com/leeforge/core/server/ent/mediafolder"
	"github.com/leeforge/core/server/ent/mediaformat"
	"github.com/leeforge/core/server/ent/predicate"
	"github.com/leeforge/core/server/pkg/errors"

	"github.com/leeforge/framework/logging"
	frameProcessor "github.com/leeforge/framework/media/processor"
	frameStorage "github.com/leeforge/framework/media/storage"
)

const (
	includeAll     = "all"
	includeFolders = "folders"
	includeFiles   = "files"

	sortByName      = "name"
	sortByCreatedAt = "createdAt"
	sortByUpdatedAt = "updatedAt"
	sortBySize      = "size"

	thumbnailFormatName = "thumbnail"
)

type UpdateMediaInput struct {
	Title      string
	AltText    string
	Caption    string
	FolderPath string
	Thumbnail  *multipart.FileHeader
}

type MediaService struct {
	client    *ent.Client
	logger    logging.Logger
	provider  frameStorage.StorageProvider
	processor frameProcessor.Resizer
}

func NewMediaService(
	client *ent.Client,
	logger logging.Logger,
	provider frameStorage.StorageProvider,
	processor frameProcessor.Resizer,
) *MediaService {
	return &MediaService{
		client:    client,
		logger:    logger,
		provider:  provider,
		processor: processor,
	}
}

func normalizeFolderPath(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "/"
	}

	p = strings.ReplaceAll(p, "\\", "/")
	clean := path.Clean(p)
	if clean == "." || clean == "" {
		clean = "/"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if clean != "/" {
		clean = strings.TrimRight(clean, "/")
	}
	return clean
}

func joinFolderPath(parentPath, name string) string {
	parentPath = normalizeFolderPath(parentPath)
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return parentPath
	}
	if parentPath == "/" {
		return "/" + name
	}
	return parentPath + "/" + name
}

func folderDepthFromPath(folderPath string) int {
	folderPath = normalizeFolderPath(folderPath)
	if folderPath == "/" {
		return 0
	}
	return len(strings.Split(strings.Trim(folderPath, "/"), "/"))
}

func folderNameFromPath(folderPath string) string {
	folderPath = normalizeFolderPath(folderPath)
	if folderPath == "/" {
		return ""
	}
	parts := strings.Split(strings.Trim(folderPath, "/"), "/")
	return parts[len(parts)-1]
}

func validateFolderName(name string) error {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return errors.NewValidationError("Folder name is required")
	}
	if strings.Contains(clean, "/") || strings.Contains(clean, "\\") {
		return errors.NewValidationError("Folder name cannot contain path separators")
	}
	return nil
}

func mediaFolderPath(m *ent.Media) string {
	if m == nil || m.FolderPath == nil {
		return "/"
	}
	return normalizeFolderPath(*m.FolderPath)
}

func normalizeListOptions(opts ListMediaLibraryOptions) ListMediaLibraryOptions {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 200 {
		opts.PageSize = 20
	}

	switch strings.ToLower(opts.Include) {
	case includeFolders:
		opts.Include = includeFolders
	case includeFiles:
		opts.Include = includeFiles
	default:
		opts.Include = includeAll
	}

	switch opts.Sort {
	case sortByName, sortByCreatedAt, sortByUpdatedAt, sortBySize:
	default:
		opts.Sort = sortByCreatedAt
	}

	if strings.EqualFold(opts.Order, "asc") {
		opts.Order = "asc"
	} else {
		opts.Order = "desc"
	}

	return opts
}

func isAsc(order string) bool {
	return strings.EqualFold(order, "asc")
}

func ptrUUID(v uuid.UUID) *uuid.UUID {
	c := v
	return &c
}

func ptrInt(v int) *int {
	c := v
	return &c
}

func ptrInt64(v int64) *int64 {
	c := v
	return &c
}

func dedupeUUIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func sameOptionalUUID(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func fileExtFromMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}

func detectMediaType(file multipart.File, header *multipart.FileHeader) (string, error) {
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", errors.NewValidationError("Failed to read uploaded file", err)
	}
	if _, seekErr := file.Seek(0, 0); seekErr != nil {
		return "", errors.NewInternalError("Failed to reset uploaded file", seekErr)
	}

	detected := http.DetectContentType(buf[:n])
	if strings.TrimSpace(contentType) == "" || strings.EqualFold(contentType, "application/octet-stream") {
		contentType = detected
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", errors.NewValidationError("Thumbnail only supports image file types")
	}
	return contentType, nil
}

func (s *MediaService) getFolderByID(ctx context.Context, id uuid.UUID) (*ent.MediaFolder, error) {
	folder, err := s.client.MediaFolder.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Folder not found", err)
		}
		return nil, errors.NewInternalError("Failed to query folder", err)
	}
	return folder, nil
}

func (s *MediaService) resolveBrowseParentID(ctx context.Context, parentID *uuid.UUID, folderPath string) (*uuid.UUID, error) {
	if parentID != nil {
		if _, err := s.getFolderByID(ctx, *parentID); err != nil {
			return nil, err
		}
		return parentID, nil
	}

	folderPath = strings.TrimSpace(folderPath)
	if folderPath == "" {
		return nil, nil
	}

	folderPath = normalizeFolderPath(folderPath)
	if folderPath == "/" {
		return nil, nil
	}

	folder, err := s.client.MediaFolder.Query().Where(mediafolder.PathEQ(folderPath)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.NewInternalError("Failed to resolve folder path", err)
	}

	return ptrUUID(folder.ID), nil
}

func (s *MediaService) ensureFolderByPath(ctx context.Context, folderPath string) (*ent.MediaFolder, error) {
	folderPath = normalizeFolderPath(folderPath)
	if folderPath == "/" {
		return nil, nil
	}

	segments := strings.Split(strings.Trim(folderPath, "/"), "/")
	parentPath := "/"
	var parentID *uuid.UUID
	var current *ent.MediaFolder

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		nextPath := joinFolderPath(parentPath, seg)
		folder, err := s.client.MediaFolder.Query().Where(mediafolder.PathEQ(nextPath)).Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, errors.NewInternalError("Failed to query folder by path", err)
		}

		if ent.IsNotFound(err) {
			builder := s.client.MediaFolder.Create().
				SetName(seg).
				SetPath(nextPath).
				SetDepth(folderDepthFromPath(nextPath))
			if parentID != nil {
				builder.SetParentID(*parentID)
			}

			created, createErr := builder.Save(ctx)
			if createErr != nil {
				// Handle racing creates by re-querying.
				if ent.IsConstraintError(createErr) {
					created, createErr = s.client.MediaFolder.Query().Where(mediafolder.PathEQ(nextPath)).Only(ctx)
				}
			}
			if createErr != nil {
				return nil, errors.NewInternalError("Failed to create folder path", createErr)
			}
			folder = created
		}

		current = folder
		parentPath = nextPath
		parentID = ptrUUID(folder.ID)
	}

	return current, nil
}

// UploadFile handles file upload, processing, and database record creation
func (s *MediaService) UploadFile(ctx context.Context, header *multipart.FileHeader, ownerID uuid.UUID, folderPath string) (*ent.Media, error) {
	folderPath = normalizeFolderPath(folderPath)
	folder, err := s.ensureFolderByPath(ctx, folderPath)
	if err != nil {
		return nil, err
	}

	file, err := header.Open()
	if err != nil {
		return nil, errors.NewInternalError("Failed to open uploaded file", err)
	}
	defer file.Close()

	// Calculate hash for deduplication or integrity check (optional)
	// For now, we just use it to generate unique filename
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, errors.NewInternalError("Failed to calculate file hash", err)
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))

	// Reset file reader
	file.Seek(0, 0)

	// Generate storage path
	ext := filepath.Ext(header.Filename)
	uniqueFilename := fmt.Sprintf("%s%s", fileHash, ext)

	// Process image dimensions if applicable
	width, height := 0, 0
	isImage := strings.HasPrefix(header.Header.Get("Content-Type"), "image/")
	if isImage && s.processor != nil {
		w, h, err := s.processor.GetDimensions(file)
		if err == nil {
			width, height = w, h
		}
		file.Seek(0, 0) // Reset again
	}

	// Upload to storage provider
	uploadOut, err := s.provider.Upload(ctx, frameStorage.UploadInput{
		File:     file,
		Filename: uniqueFilename,
		Folder:   folderPath,
		Size:     header.Size,
	})
	if err != nil {
		return nil, errors.NewInternalError("Failed to upload file to storage", err)
	}

	builder := s.client.Media.Create().
		SetName(header.Filename).
		SetHash(fileHash).
		SetURL(uploadOut.URL).
		SetMime(header.Header.Get("Content-Type")).
		SetSize(float64(header.Size) / (1024 * 1024)). // Convert to MB
		SetOwnerID(ownerID).
		SetFolderPath(folderPath).
		SetWidth(width).
		SetHeight(height).
		SetProvider("local") // Or uploadOut.Metadata if available
	if folder != nil {
		builder.SetFolderID(folder.ID)
	}

	// Create database record
	m, err := builder.Save(ctx)

	if err != nil {
		s.logger.Error("Failed to save media record",
			zap.Error(err),
			zap.String("name", header.Filename),
			zap.String("folderPath", folderPath),
			zap.String("ownerID", ownerID.String()),
		)
		// Try to clean up uploaded file if DB save fails
		_ = s.provider.Delete(ctx, frameStorage.DeleteInput{
			Filename: filepath.Base(uploadOut.URL),
			Folder:   folderPath,
		})
		return nil, errors.NewInternalError("Failed to save media record", err)
	}

	// TODO: Trigger async thumbnail generation job here

	return m, nil
}

// GetMedia retrieves media details
func (s *MediaService) GetMedia(ctx context.Context, id uuid.UUID) (*ent.Media, error) {
	m, err := s.client.Media.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Media not found", err)
		}
		return nil, errors.NewInternalError("Failed to query media", err)
	}
	return m, nil
}

// DeleteMedia removes media from DB and storage
func (s *MediaService) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	m, err := s.client.Media.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Media not found", err)
		}
		return errors.NewInternalError("Failed to query media", err)
	}

	// Delete from storage first (best effort)
	if s.provider != nil {
		if err := s.provider.Delete(ctx, frameStorage.DeleteInput{
			Filename: filepath.Base(m.URL),
			Folder:   mediaFolderPath(m),
		}); err != nil {
			s.logger.Warn("Failed to delete file from storage", zap.String("url", m.URL), zap.Error(err))
			// Continue to delete from DB even if storage deletion fails (consistency)
		}
	}

	// Delete from DB
	if err := s.client.Media.DeleteOneID(m.ID).Exec(ctx); err != nil {
		return errors.NewInternalError("Failed to delete media record", err)
	}

	return nil
}

// GetSignedURL generates a temporary access URL
func (s *MediaService) GetSignedURL(ctx context.Context, id uuid.UUID, expiry time.Duration) (string, error) {
	if s.provider == nil {
		return "", errors.NewInternalError("Storage provider is not configured", nil)
	}

	m, err := s.client.Media.Get(ctx, id)
	if err != nil {
		return "", errors.NewInternalError("Failed to query media", err)
	}

	return s.provider.GetSignedURL(ctx, frameStorage.GetSignedURLInput{
		Filename: filepath.Base(m.URL),
		Folder:   mediaFolderPath(m),
		Expires:  int64(expiry.Seconds()),
	})
}

// UpdateMedia updates metadata.
func (s *MediaService) UpdateMedia(
	ctx context.Context,
	id uuid.UUID,
	title, altText, caption, folderPath string,
) (*ent.Media, error) {
	return s.UpdateMediaWithInput(ctx, id, UpdateMediaInput{
		Title:      title,
		AltText:    altText,
		Caption:    caption,
		FolderPath: folderPath,
	})
}

// UpdateMediaWithInput updates media metadata and optional thumbnail content.
func (s *MediaService) UpdateMediaWithInput(
	ctx context.Context,
	id uuid.UUID,
	input UpdateMediaInput,
) (*ent.Media, error) {
	current, err := s.client.Media.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Media not found", err)
		}
		return nil, errors.NewInternalError("Failed to query media", err)
	}

	targetName := current.Name
	if input.Title != "" {
		targetName = input.Title
	}

	targetFolderID := current.FolderID
	targetFolderPath := mediaFolderPath(current)
	if input.FolderPath != "" {
		normalizedPath := normalizeFolderPath(input.FolderPath)
		folder, folderErr := s.ensureFolderByPath(ctx, normalizedPath)
		if folderErr != nil {
			return nil, folderErr
		}
		targetFolderPath = normalizedPath
		if folder != nil {
			targetFolderID = ptrUUID(folder.ID)
		} else {
			targetFolderID = nil
		}
	}

	if targetName != current.Name || !sameOptionalUUID(targetFolderID, current.FolderID) {
		conflictQuery := s.client.Media.Query().Where(
			media.NameEQ(targetName),
			media.IDNEQ(current.ID),
		)
		if targetFolderID == nil {
			conflictQuery.Where(media.FolderIDIsNil())
		} else {
			conflictQuery.Where(media.FolderIDEQ(*targetFolderID))
		}

		exists, conflictErr := conflictQuery.Exist(ctx)
		if conflictErr != nil {
			return nil, errors.NewInternalError("Failed to check media name conflict", conflictErr)
		}
		if exists {
			return nil, errors.NewConflictError("File name already exists under destination")
		}
	}

	if input.Thumbnail != nil && s.provider == nil {
		return nil, errors.NewInternalError("Storage provider is not configured", nil)
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	updater := tx.Media.UpdateOneID(id)
	if input.Title != "" {
		updater.SetName(input.Title)
	}
	if input.AltText != "" {
		updater.SetAlternativeText(input.AltText)
	}
	if input.Caption != "" {
		updater.SetCaption(input.Caption)
	}
	if input.FolderPath != "" {
		updater.SetFolderPath(targetFolderPath)
		if targetFolderID != nil {
			updater.SetFolderID(*targetFolderID)
		} else {
			updater.ClearFolderID()
		}
	}

	updated, err := updater.Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to update media", err)
	}

	var (
		uploadedThumbnailDeleteInput *frameStorage.DeleteInput
		oldThumbnailForCleanup       *ent.MediaFormat
	)
	if input.Thumbnail != nil {
		oldThumbnailForCleanup, uploadedThumbnailDeleteInput, err = s.upsertThumbnailFormatTx(ctx, tx, updated, input.Thumbnail)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		if uploadedThumbnailDeleteInput != nil && s.provider != nil {
			_ = s.provider.Delete(ctx, *uploadedThumbnailDeleteInput)
		}
		return nil, errors.NewInternalError("Failed to commit media update", err)
	}

	if oldThumbnailForCleanup != nil && uploadedThumbnailDeleteInput != nil && s.provider != nil {
		oldFilename := filepath.Base(oldThumbnailForCleanup.URL)
		oldFolder := "/"
		if oldThumbnailForCleanup.Path != nil && strings.TrimSpace(*oldThumbnailForCleanup.Path) != "" {
			oldFolder = normalizeFolderPath(*oldThumbnailForCleanup.Path)
		}

		sameStoredFile := oldFilename == uploadedThumbnailDeleteInput.Filename &&
			normalizeFolderPath(uploadedThumbnailDeleteInput.Folder) == oldFolder
		if !sameStoredFile {
			s.deleteMediaFormatFileBestEffort(ctx, oldThumbnailForCleanup)
		}
	}

	return updated, nil
}

func (s *MediaService) upsertThumbnailFormatTx(
	ctx context.Context,
	tx *ent.Tx,
	m *ent.Media,
	header *multipart.FileHeader,
) (*ent.MediaFormat, *frameStorage.DeleteInput, error) {
	file, err := header.Open()
	if err != nil {
		return nil, nil, errors.NewInternalError("Failed to open thumbnail file", err)
	}
	defer file.Close()

	contentType, err := detectMediaType(file, header)
	if err != nil {
		return nil, nil, err
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, nil, errors.NewInternalError("Failed to calculate thumbnail hash", err)
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, 0); err != nil {
		return nil, nil, errors.NewInternalError("Failed to reset thumbnail file", err)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = fileExtFromMime(contentType)
	}
	uniqueFilename := fmt.Sprintf("%s%s", fileHash, ext)
	thumbnailFolder := joinFolderPath(mediaFolderPath(m), ".thumbnails")

	width, height := 0, 0
	if s.processor != nil {
		w, h, dimErr := s.processor.GetDimensions(file)
		if dimErr == nil {
			width = w
			height = h
		}
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return nil, nil, errors.NewInternalError("Failed to reset thumbnail file", seekErr)
		}
	}

	uploadOut, err := s.provider.Upload(ctx, frameStorage.UploadInput{
		File:     file,
		Filename: uniqueFilename,
		Folder:   thumbnailFolder,
		Size:     header.Size,
	})
	if err != nil {
		return nil, nil, errors.NewInternalError("Failed to upload thumbnail", err)
	}

	sizeInBytes := uploadOut.Size
	if sizeInBytes <= 0 {
		sizeInBytes = header.Size
	}
	var sizeMB *float64
	if sizeInBytes > 0 {
		v := float64(sizeInBytes) / (1024 * 1024)
		sizeMB = &v
	}

	existingFormats, err := tx.MediaFormat.Query().
		Where(
			mediaformat.MediaIDEQ(m.ID),
			mediaformat.NameEQ(thumbnailFormatName),
		).
		Order(ent.Desc(mediaformat.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, errors.NewInternalError("Failed to query thumbnail format", err)
	}

	var oldFormatForCleanup *ent.MediaFormat
	if len(existingFormats) > 0 {
		old := existingFormats[0]
		oldFormatForCleanup = old

		updater := tx.MediaFormat.UpdateOneID(old.ID).
			SetURL(uploadOut.URL).
			SetPath(thumbnailFolder).
			SetExt(ext).
			SetMime(contentType).
			SetSizeInBytes(sizeInBytes)
		if sizeMB != nil {
			updater.SetSize(*sizeMB)
		}
		if width > 0 {
			updater.SetWidth(width)
		}
		if height > 0 {
			updater.SetHeight(height)
		}
		if _, err := updater.Save(ctx); err != nil {
			return nil, nil, errors.NewInternalError("Failed to update thumbnail format", err)
		}

		if len(existingFormats) > 1 {
			duplicateIDs := make([]uuid.UUID, 0, len(existingFormats)-1)
			for _, duplicated := range existingFormats[1:] {
				duplicateIDs = append(duplicateIDs, duplicated.ID)
			}
			if len(duplicateIDs) > 0 {
				if _, err := tx.MediaFormat.Delete().Where(mediaformat.IDIn(duplicateIDs...)).Exec(ctx); err != nil {
					return nil, nil, errors.NewInternalError("Failed to clean duplicate thumbnail formats", err)
				}
			}
		}
	} else {
		builder := tx.MediaFormat.Create().
			SetName(thumbnailFormatName).
			SetURL(uploadOut.URL).
			SetPath(thumbnailFolder).
			SetMediaID(m.ID).
			SetExt(ext).
			SetMime(contentType).
			SetSizeInBytes(sizeInBytes)
		if sizeMB != nil {
			builder.SetSize(*sizeMB)
		}
		if width > 0 {
			builder.SetWidth(width)
		}
		if height > 0 {
			builder.SetHeight(height)
		}
		if _, err := builder.Save(ctx); err != nil {
			return nil, nil, errors.NewInternalError("Failed to create thumbnail format", err)
		}
	}

	deleteInput := &frameStorage.DeleteInput{
		Filename: filepath.Base(uploadOut.URL),
		Folder:   thumbnailFolder,
	}
	return oldFormatForCleanup, deleteInput, nil
}

func (s *MediaService) deleteMediaFormatFileBestEffort(ctx context.Context, f *ent.MediaFormat) {
	if f == nil || s.provider == nil {
		return
	}

	filename := filepath.Base(f.URL)
	if filename == "" || filename == "." || filename == "/" {
		return
	}

	folder := "/"
	if f.Path != nil && strings.TrimSpace(*f.Path) != "" {
		folder = normalizeFolderPath(*f.Path)
	}

	if err := s.provider.Delete(ctx, frameStorage.DeleteInput{
		Filename: filename,
		Folder:   folder,
	}); err != nil {
		s.logger.Warn("Failed to delete old thumbnail from storage", zap.String("url", f.URL), zap.Error(err))
	}
}

// ListMediaLibrary returns mixed folder/file nodes for hierarchical media browsing.
func (s *MediaService) ListMediaLibrary(ctx context.Context, opts ListMediaLibraryOptions) ([]MediaLibraryNode, int, error) {
	opts = normalizeListOptions(opts)

	parentID, err := s.resolveBrowseParentID(ctx, opts.ParentID, opts.Path)
	if err != nil {
		return nil, 0, err
	}

	nodes := make([]MediaLibraryNode, 0, opts.PageSize)

	wantFolders := opts.Include == includeAll || opts.Include == includeFolders
	wantFiles := opts.Include == includeAll || opts.Include == includeFiles

	search := strings.TrimSpace(opts.Search)
	mimePrefix := strings.TrimSpace(opts.MimePrefix)
	provider := strings.TrimSpace(opts.Provider)

	if wantFolders {
		query := s.client.MediaFolder.Query()
		if parentID == nil {
			query.Where(mediafolder.ParentIDIsNil())
		} else {
			query.Where(mediafolder.ParentIDEQ(*parentID))
		}
		if search != "" {
			query.Where(mediafolder.NameContainsFold(search))
		}

		folders, queryErr := query.All(ctx)
		if queryErr != nil {
			return nil, 0, errors.NewInternalError("Failed to query folders", queryErr)
		}

		for _, folder := range folders {
			childrenCount, childErr := s.client.MediaFolder.Query().
				Where(mediafolder.ParentIDEQ(folder.ID)).
				Count(ctx)
			if childErr != nil {
				return nil, 0, errors.NewInternalError("Failed to count child folders", childErr)
			}

			filesInFolder, fileErr := s.client.Media.Query().
				Where(media.FolderIDEQ(folder.ID)).
				All(ctx)
			if fileErr != nil {
				return nil, 0, errors.NewInternalError("Failed to query folder files", fileErr)
			}

			var totalBytes int64
			for _, mf := range filesInFolder {
				if mf.Size != nil {
					totalBytes += int64((*mf.Size) * 1024 * 1024)
				}
			}

			nodes = append(nodes, MediaLibraryNode{
				ID:            folder.ID,
				Kind:          "folder",
				Name:          folder.Name,
				ParentID:      folder.ParentID,
				Path:          folder.Path,
				CreatedAt:     folder.CreatedAt,
				UpdatedAt:     folder.UpdatedAt,
				ChildrenCount: ptrInt(childrenCount),
				FilesCount:    ptrInt(len(filesInFolder)),
				SizeBytes:     ptrInt64(totalBytes),
			})
		}
	}

	if wantFiles {
		query := s.client.Media.Query()
		if parentID == nil {
			query.Where(media.FolderIDIsNil())
		} else {
			query.Where(media.FolderIDEQ(*parentID))
		}
		if search != "" {
			query.Where(media.Or(media.NameContainsFold(search), media.AliasContainsFold(search)))
		}
		if mimePrefix != "" {
			query.Where(media.MimeHasPrefix(mimePrefix))
		}
		if provider != "" {
			query.Where(media.ProviderEQ(provider))
		}

		files, queryErr := query.All(ctx)
		if queryErr != nil {
			return nil, 0, errors.NewInternalError("Failed to query media files", queryErr)
		}

		for _, mf := range files {
			nodes = append(nodes, MediaLibraryNode{
				ID:        mf.ID,
				Kind:      "file",
				Name:      mf.Name,
				ParentID:  mf.FolderID,
				Path:      mediaFolderPath(mf),
				CreatedAt: mf.CreatedAt,
				UpdatedAt: mf.UpdatedAt,
				Mime:      mf.Mime,
				Size:      mf.Size,
				URL:       &mf.URL,
				Width:     mf.Width,
				Height:    mf.Height,
				Provider:  mf.Provider,
				Alias:     mf.Alias,
			})
		}
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		li := nodes[i]
		lj := nodes[j]

		if opts.FoldersFirst && li.Kind != lj.Kind {
			return li.Kind == "folder"
		}

		less := false
		switch opts.Sort {
		case sortByName:
			liName := strings.ToLower(li.Name)
			ljName := strings.ToLower(lj.Name)
			if liName == ljName {
				less = li.ID.String() < lj.ID.String()
			} else {
				less = liName < ljName
			}
		case sortByUpdatedAt:
			if li.UpdatedAt.Equal(lj.UpdatedAt) {
				less = li.ID.String() < lj.ID.String()
			} else {
				less = li.UpdatedAt.Before(lj.UpdatedAt)
			}
		case sortBySize:
			var lSize int64
			if li.Kind == "folder" && li.SizeBytes != nil {
				lSize = *li.SizeBytes
			}
			if li.Kind == "file" && li.Size != nil {
				lSize = int64(*li.Size * 1024 * 1024)
			}
			var rSize int64
			if lj.Kind == "folder" && lj.SizeBytes != nil {
				rSize = *lj.SizeBytes
			}
			if lj.Kind == "file" && lj.Size != nil {
				rSize = int64(*lj.Size * 1024 * 1024)
			}
			if lSize == rSize {
				less = li.ID.String() < lj.ID.String()
			} else {
				less = lSize < rSize
			}
		default: // createdAt
			if li.CreatedAt.Equal(lj.CreatedAt) {
				less = li.ID.String() < lj.ID.String()
			} else {
				less = li.CreatedAt.Before(lj.CreatedAt)
			}
		}

		if isAsc(opts.Order) {
			return less
		}
		return !less
	})

	total := len(nodes)
	offset := (opts.Page - 1) * opts.PageSize
	if offset >= total {
		return []MediaLibraryNode{}, total, nil
	}

	end := offset + opts.PageSize
	if end > total {
		end = total
	}

	return nodes[offset:end], total, nil
}

func (s *MediaService) siblingNameExists(
	ctx context.Context,
	tx *ent.Tx,
	parentID *uuid.UUID,
	name string,
	excludeID *uuid.UUID,
) (bool, error) {
	query := tx.MediaFolder.Query().Where(mediafolder.NameEQ(name))
	if parentID == nil {
		query.Where(mediafolder.ParentIDIsNil())
	} else {
		query.Where(mediafolder.ParentIDEQ(*parentID))
	}
	if excludeID != nil {
		query.Where(mediafolder.IDNEQ(*excludeID))
	}

	count, err := query.Count(ctx)
	if err != nil {
		return false, errors.NewInternalError("Failed to check folder sibling conflicts", err)
	}
	return count > 0, nil
}

func (s *MediaService) CreateFolder(ctx context.Context, input CreateFolderInput) (*ent.MediaFolder, error) {
	if strings.TrimSpace(input.Path) != "" {
		folderPath := normalizeFolderPath(input.Path)
		if folderPath == "/" {
			return nil, errors.NewValidationError("Root path cannot be created explicitly")
		}

		existing, err := s.client.MediaFolder.Query().Where(mediafolder.PathEQ(folderPath)).Only(ctx)
		if err == nil && existing != nil {
			return nil, errors.NewConflictError("Folder already exists")
		}
		if err != nil && !ent.IsNotFound(err) {
			return nil, errors.NewInternalError("Failed to query folder path", err)
		}

		return s.ensureFolderByPath(ctx, folderPath)
	}

	if err := validateFolderName(input.Name); err != nil {
		return nil, err
	}

	var (
		parentPath string
		parentID   *uuid.UUID
	)
	if input.ParentID != nil {
		parent, err := s.getFolderByID(ctx, *input.ParentID)
		if err != nil {
			return nil, err
		}
		parentPath = parent.Path
		parentID = ptrUUID(parent.ID)
	} else {
		parentPath = "/"
	}

	newPath := joinFolderPath(parentPath, input.Name)

	existing, err := s.client.MediaFolder.Query().Where(mediafolder.PathEQ(newPath)).Only(ctx)
	if err == nil && existing != nil {
		return nil, errors.NewConflictError("Folder already exists")
	}
	if err != nil && !ent.IsNotFound(err) {
		return nil, errors.NewInternalError("Failed to check folder path conflict", err)
	}

	builder := s.client.MediaFolder.Create().
		SetName(strings.TrimSpace(input.Name)).
		SetPath(newPath).
		SetDepth(folderDepthFromPath(newPath))
	if parentID != nil {
		builder.SetParentID(*parentID)
	}

	folder, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Folder already exists", err)
		}
		return nil, errors.NewInternalError("Failed to create folder", err)
	}

	return folder, nil
}

func (s *MediaService) relocateFolderTreeTx(
	ctx context.Context,
	tx *ent.Tx,
	folder *ent.MediaFolder,
	newParentID *uuid.UUID,
	newName string,
) (*ent.MediaFolder, error) {
	parentPath := "/"
	if newParentID != nil {
		parent, err := tx.MediaFolder.Get(ctx, *newParentID)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, errors.NewNotFoundError("Destination folder not found", err)
			}
			return nil, errors.NewInternalError("Failed to query destination folder", err)
		}
		parentPath = parent.Path
	}

	oldPath := folder.Path
	newPath := joinFolderPath(parentPath, newName)
	newDepth := folderDepthFromPath(newPath)

	update := tx.MediaFolder.UpdateOneID(folder.ID).
		SetName(newName).
		SetPath(newPath).
		SetDepth(newDepth)
	if newParentID != nil {
		update.SetParentID(*newParentID)
	} else {
		update.ClearParentID()
		update.ClearParent()
	}

	updated, err := update.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Folder name already exists under destination", err)
		}
		return nil, errors.NewInternalError("Failed to update folder", err)
	}

	descendants, err := tx.MediaFolder.Query().
		Where(mediafolder.PathHasPrefix(oldPath + "/")).
		All(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to query descendant folders", err)
	}

	folderPathByID := map[uuid.UUID]string{
		updated.ID: newPath,
	}

	for _, child := range descendants {
		suffix := strings.TrimPrefix(child.Path, oldPath)
		childPath := normalizeFolderPath(newPath + suffix)
		_, updateErr := tx.MediaFolder.UpdateOneID(child.ID).
			SetPath(childPath).
			SetDepth(folderDepthFromPath(childPath)).
			Save(ctx)
		if updateErr != nil {
			return nil, errors.NewInternalError("Failed to update descendant folder path", updateErr)
		}
		folderPathByID[child.ID] = childPath
	}

	for folderID, folderPath := range folderPathByID {
		if _, updateErr := tx.Media.Update().
			Where(media.FolderIDEQ(folderID)).
			SetFolderPath(folderPath).
			Save(ctx); updateErr != nil {
			return nil, errors.NewInternalError("Failed to sync media folder path", updateErr)
		}
	}

	return updated, nil
}

func (s *MediaService) UpdateFolder(ctx context.Context, id uuid.UUID, input UpdateFolderInput) (*ent.MediaFolder, error) {
	current, err := s.getFolderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newName := current.Name
	if input.Name != nil {
		if err := validateFolderName(*input.Name); err != nil {
			return nil, err
		}
		newName = strings.TrimSpace(*input.Name)
	}

	targetParentID := current.ParentID
	parentChanged := false
	if input.MoveToRoot {
		targetParentID = nil
		parentChanged = true
	} else if input.ParentID != nil {
		parentChanged = true
		targetParentID = input.ParentID
	}

	if targetParentID != nil {
		if *targetParentID == current.ID {
			return nil, errors.NewValidationError("Folder cannot be moved into itself")
		}
		parentFolder, parentErr := s.getFolderByID(ctx, *targetParentID)
		if parentErr != nil {
			return nil, parentErr
		}
		if strings.HasPrefix(parentFolder.Path, current.Path+"/") {
			return nil, errors.NewValidationError("Folder cannot be moved into its descendant")
		}
	}

	if !parentChanged && newName == current.Name {
		return current, nil
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	duplicate, dupErr := s.siblingNameExists(ctx, tx, targetParentID, newName, &current.ID)
	if dupErr != nil {
		return nil, dupErr
	}
	if duplicate {
		return nil, errors.NewConflictError("Folder name already exists under destination")
	}

	updated, err := s.relocateFolderTreeTx(ctx, tx, current, targetParentID, newName)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit folder update", err)
	}

	return updated, nil
}

func (s *MediaService) collectFolderSubtreeTx(
	ctx context.Context,
	tx *ent.Tx,
	folder *ent.MediaFolder,
) ([]*ent.MediaFolder, error) {
	descendants, err := tx.MediaFolder.Query().
		Where(mediafolder.PathHasPrefix(folder.Path + "/")).
		All(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to query folder descendants", err)
	}

	out := make([]*ent.MediaFolder, 0, len(descendants)+1)
	out = append(out, folder)
	out = append(out, descendants...)
	return out, nil
}

func (s *MediaService) DeleteFolder(ctx context.Context, id uuid.UUID, recursive bool) (*FolderDeleteResult, error) {
	folder, err := s.getFolderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if !recursive {
		childrenCount, childErr := tx.MediaFolder.Query().
			Where(mediafolder.ParentIDEQ(folder.ID)).
			Count(ctx)
		if childErr != nil {
			return nil, errors.NewInternalError("Failed to check folder children", childErr)
		}
		filesCount, fileErr := tx.Media.Query().
			Where(media.FolderIDEQ(folder.ID)).
			Count(ctx)
		if fileErr != nil {
			return nil, errors.NewInternalError("Failed to check folder files", fileErr)
		}
		if childrenCount > 0 || filesCount > 0 {
			return nil, errors.NewConflictError("Folder is not empty, set recursive=true to delete")
		}
	}

	targetFolders := []*ent.MediaFolder{folder}
	if recursive {
		subtree, subtreeErr := s.collectFolderSubtreeTx(ctx, tx, folder)
		if subtreeErr != nil {
			return nil, subtreeErr
		}
		targetFolders = subtree
	}

	folderIDs := make([]uuid.UUID, 0, len(targetFolders))
	for _, f := range targetFolders {
		folderIDs = append(folderIDs, f.ID)
	}

	var files []*ent.Media
	if len(folderIDs) > 0 {
		files, err = tx.Media.Query().Where(media.FolderIDIn(folderIDs...)).All(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to query files for folder deletion", err)
		}
	}

	for _, mf := range files {
		if s.provider != nil {
			if delErr := s.provider.Delete(ctx, frameStorage.DeleteInput{
				Filename: filepath.Base(mf.URL),
				Folder:   mediaFolderPath(mf),
			}); delErr != nil {
				s.logger.Warn("Failed to delete file from storage", zap.String("url", mf.URL), zap.Error(delErr))
			}
		}
	}

	fileIDs := make([]uuid.UUID, 0, len(files))
	for _, mf := range files {
		fileIDs = append(fileIDs, mf.ID)
	}
	if len(fileIDs) > 0 {
		if _, err := tx.Media.Delete().Where(media.IDIn(fileIDs...)).Exec(ctx); err != nil {
			return nil, errors.NewInternalError("Failed to delete media files", err)
		}
	}

	sort.Slice(targetFolders, func(i, j int) bool {
		return targetFolders[i].Depth > targetFolders[j].Depth
	})
	for _, f := range targetFolders {
		if err := tx.MediaFolder.DeleteOneID(f.ID).Exec(ctx); err != nil {
			return nil, errors.NewInternalError("Failed to delete folder", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit folder deletion", err)
	}

	return &FolderDeleteResult{
		DeletedFolders: len(targetFolders),
		DeletedFiles:   len(fileIDs),
	}, nil
}

func (s *MediaService) BulkMove(ctx context.Context, input BulkMoveInput) (*BulkMoveResult, error) {
	input.FileIDs = dedupeUUIDs(input.FileIDs)
	input.FolderIDs = dedupeUUIDs(input.FolderIDs)

	if input.DestinationRoot && input.DestinationFolderID != nil {
		return nil, errors.NewValidationError("destinationRoot and destinationFolderId cannot be used together")
	}

	if len(input.FileIDs) == 0 && len(input.FolderIDs) == 0 {
		return &BulkMoveResult{}, nil
	}

	var (
		destID   *uuid.UUID
		destPath = "/"
	)
	if !input.DestinationRoot && input.DestinationFolderID != nil {
		destID = input.DestinationFolderID
		destFolder, err := s.getFolderByID(ctx, *destID)
		if err != nil {
			return nil, err
		}
		destPath = destFolder.Path
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	folders := make([]*ent.MediaFolder, 0, len(input.FolderIDs))
	for _, folderID := range input.FolderIDs {
		folder, folderErr := tx.MediaFolder.Get(ctx, folderID)
		if folderErr != nil {
			if ent.IsNotFound(folderErr) {
				return nil, errors.NewNotFoundError("Folder not found", folderErr)
			}
			return nil, errors.NewInternalError("Failed to query folder", folderErr)
		}

		if destID != nil {
			if folder.ID == *destID {
				return nil, errors.NewValidationError("Folder cannot be moved into itself")
			}
			if strings.HasPrefix(destPath, folder.Path+"/") {
				return nil, errors.NewValidationError("Folder cannot be moved into its descendant")
			}
		}

		folders = append(folders, folder)
	}

	// Keep only top-level folders in selection (moving parent already moves descendants).
	sort.Slice(folders, func(i, j int) bool { return folders[i].Depth < folders[j].Depth })
	selected := make([]*ent.MediaFolder, 0, len(folders))
	for _, folder := range folders {
		skipped := false
		for _, already := range selected {
			if strings.HasPrefix(folder.Path, already.Path+"/") {
				skipped = true
				break
			}
		}
		if !skipped {
			selected = append(selected, folder)
		}
	}

	selectedIDs := make(map[uuid.UUID]struct{}, len(selected))
	for _, folder := range selected {
		selectedIDs[folder.ID] = struct{}{}
	}

	seenNames := make(map[string]struct{}, len(selected))
	for _, folder := range selected {
		key := strings.ToLower(folder.Name)
		if _, ok := seenNames[key]; ok {
			return nil, errors.NewConflictError("Folder name conflict in move selection")
		}
		seenNames[key] = struct{}{}
	}

	for _, folder := range selected {
		conflictQuery := tx.MediaFolder.Query().Where(
			mediafolder.NameEQ(folder.Name),
			mediafolder.IDNEQ(folder.ID),
		)
		if destID == nil {
			conflictQuery.Where(mediafolder.ParentIDIsNil())
		} else {
			conflictQuery.Where(mediafolder.ParentIDEQ(*destID))
		}

		conflicts, queryErr := conflictQuery.All(ctx)
		if queryErr != nil {
			return nil, errors.NewInternalError("Failed to query sibling conflicts", queryErr)
		}

		for _, conflict := range conflicts {
			if _, selectedConflict := selectedIDs[conflict.ID]; selectedConflict {
				continue
			}
			return nil, errors.NewConflictError("Folder name already exists under destination")
		}
	}

	for _, folder := range selected {
		if _, moveErr := s.relocateFolderTreeTx(ctx, tx, folder, destID, folder.Name); moveErr != nil {
			return nil, moveErr
		}
	}

	fileIDsToMove := make([]uuid.UUID, 0, len(input.FileIDs))
	if len(input.FileIDs) > 0 {
		files, fileQueryErr := tx.Media.Query().Where(media.IDIn(input.FileIDs...)).All(ctx)
		if fileQueryErr != nil {
			return nil, errors.NewInternalError("Failed to query files for move", fileQueryErr)
		}

		filtered := make([]*ent.Media, 0, len(files))
		for _, f := range files {
			coveredBySelectedFolder := false
			fileFolderPath := mediaFolderPath(f)
			for _, folder := range selected {
				if fileFolderPath == folder.Path || strings.HasPrefix(fileFolderPath, folder.Path+"/") {
					coveredBySelectedFolder = true
					break
				}
			}
			if !coveredBySelectedFolder {
				filtered = append(filtered, f)
			}
		}

		if len(filtered) > 0 {
			seenFileNames := make(map[string]struct{}, len(filtered))
			namePredicates := make([]predicate.Media, 0, len(filtered))
			for _, f := range filtered {
				fileIDsToMove = append(fileIDsToMove, f.ID)
				nameKey := strings.ToLower(strings.TrimSpace(f.Name))
				if _, ok := seenFileNames[nameKey]; ok {
					return nil, errors.NewConflictError("File name conflict in move selection")
				}
				seenFileNames[nameKey] = struct{}{}
				namePredicates = append(namePredicates, media.NameEQ(f.Name))
			}

			conflictQuery := tx.Media.Query().Where(
				media.Or(namePredicates...),
				media.IDNotIn(fileIDsToMove...),
			)
			if destID == nil {
				conflictQuery.Where(media.FolderIDIsNil())
			} else {
				conflictQuery.Where(media.FolderIDEQ(*destID))
			}

			conflictExists, conflictErr := conflictQuery.Exist(ctx)
			if conflictErr != nil {
				return nil, errors.NewInternalError("Failed to check file conflicts in destination", conflictErr)
			}
			if conflictExists {
				return nil, errors.NewConflictError("File name already exists under destination")
			}
		}
	}

	movedFiles := 0
	if len(fileIDsToMove) > 0 {
		updater := tx.Media.Update().Where(media.IDIn(fileIDsToMove...))
		if destID == nil {
			updater.ClearFolderID().SetFolderPath("/")
		} else {
			updater.SetFolderID(*destID).SetFolderPath(destPath)
		}
		movedFiles, err = updater.Save(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to move files", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit bulk move", err)
	}

	return &BulkMoveResult{
		MovedFiles:   movedFiles,
		MovedFolders: len(selected),
	}, nil
}

func (s *MediaService) BulkDelete(ctx context.Context, input BulkDeleteInput) (*BulkDeleteResult, error) {
	input.FileIDs = dedupeUUIDs(input.FileIDs)
	input.FolderIDs = dedupeUUIDs(input.FolderIDs)

	if len(input.FileIDs) == 0 && len(input.FolderIDs) == 0 {
		return &BulkDeleteResult{}, nil
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	targetFolderMap := make(map[uuid.UUID]*ent.MediaFolder)
	for _, folderID := range input.FolderIDs {
		folder, folderErr := tx.MediaFolder.Get(ctx, folderID)
		if folderErr != nil {
			if ent.IsNotFound(folderErr) {
				return nil, errors.NewNotFoundError("Folder not found", folderErr)
			}
			return nil, errors.NewInternalError("Failed to query folder", folderErr)
		}

		if !input.Recursive {
			childrenCount, childErr := tx.MediaFolder.Query().
				Where(mediafolder.ParentIDEQ(folder.ID)).
				Count(ctx)
			if childErr != nil {
				return nil, errors.NewInternalError("Failed to check folder children", childErr)
			}
			fileCount, fileErr := tx.Media.Query().
				Where(media.FolderIDEQ(folder.ID)).
				Count(ctx)
			if fileErr != nil {
				return nil, errors.NewInternalError("Failed to check folder files", fileErr)
			}
			if childrenCount > 0 || fileCount > 0 {
				return nil, errors.NewConflictError("Folder is not empty, set recursive=true to delete")
			}
			targetFolderMap[folder.ID] = folder
			continue
		}

		subtree, subtreeErr := s.collectFolderSubtreeTx(ctx, tx, folder)
		if subtreeErr != nil {
			return nil, subtreeErr
		}
		for _, f := range subtree {
			targetFolderMap[f.ID] = f
		}
	}

	targetFolderIDs := make([]uuid.UUID, 0, len(targetFolderMap))
	for folderID := range targetFolderMap {
		targetFolderIDs = append(targetFolderIDs, folderID)
	}

	fileIDSet := make(map[uuid.UUID]struct{}, len(input.FileIDs))
	for _, fileID := range input.FileIDs {
		fileIDSet[fileID] = struct{}{}
	}
	if len(targetFolderIDs) > 0 {
		folderFiles, folderFilesErr := tx.Media.Query().
			Where(media.FolderIDIn(targetFolderIDs...)).
			All(ctx)
		if folderFilesErr != nil {
			return nil, errors.NewInternalError("Failed to query files from selected folders", folderFilesErr)
		}
		for _, mf := range folderFiles {
			fileIDSet[mf.ID] = struct{}{}
		}
	}

	allFileIDs := make([]uuid.UUID, 0, len(fileIDSet))
	for fileID := range fileIDSet {
		allFileIDs = append(allFileIDs, fileID)
	}

	var files []*ent.Media
	if len(allFileIDs) > 0 {
		files, err = tx.Media.Query().Where(media.IDIn(allFileIDs...)).All(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to query files for deletion", err)
		}
	}

	for _, mf := range files {
		if s.provider != nil {
			if delErr := s.provider.Delete(ctx, frameStorage.DeleteInput{
				Filename: filepath.Base(mf.URL),
				Folder:   mediaFolderPath(mf),
			}); delErr != nil {
				s.logger.Warn("Failed to delete file from storage", zap.String("url", mf.URL), zap.Error(delErr))
			}
		}
	}

	deletedFiles := 0
	if len(allFileIDs) > 0 {
		deletedFiles, err = tx.Media.Delete().Where(media.IDIn(allFileIDs...)).Exec(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to delete files", err)
		}
	}

	targetFolders := make([]*ent.MediaFolder, 0, len(targetFolderMap))
	for _, folder := range targetFolderMap {
		targetFolders = append(targetFolders, folder)
	}
	sort.Slice(targetFolders, func(i, j int) bool {
		return targetFolders[i].Depth > targetFolders[j].Depth
	})

	deletedFolders := 0
	for _, folder := range targetFolders {
		if delErr := tx.MediaFolder.DeleteOneID(folder.ID).Exec(ctx); delErr != nil {
			return nil, errors.NewInternalError("Failed to delete folder", delErr)
		}
		deletedFolders++
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit bulk delete", err)
	}

	return &BulkDeleteResult{
		DeletedFiles:   deletedFiles,
		DeletedFolders: deletedFolders,
	}, nil
}
