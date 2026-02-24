package media

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	frameProcessor "github.com/leeforge/framework/media/processor"
	frameStorage "github.com/leeforge/framework/media/storage"
	framePerm "github.com/leeforge/framework/permission"
)

type MediaModule struct {
	logger  logging.Logger
	handler *MediaHandler
}

func NewMediaModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize infrastructure (In a real app, these might come from deps or config)
	// For MVP migration, we initialize them here as they were in service.go
	storageProvider, _ := frameStorage.NewLocalStorageProvider("./uploads", "http://localhost:8080/uploads")
	imageProcessor := frameProcessor.NewNativeProcessor()

	// Initialize Service
	svc := NewMediaService(deps.Client, logger, storageProvider, imageProcessor)

	// Initialize Handler
	handler := NewMediaHandler(svc, logger)

	return &MediaModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *MediaModule) Name() string {
	return "media"
}

// RegisterPublicRoutes - media module has no public routes
func (m *MediaModule) RegisterPublicRoutes(r chi.Router) {
	// No public routes for media module
}

// RegisterPrivateRoutes registers protected media endpoints
func (m *MediaModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/media", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.ListMedia, framePerm.Private("List media", "media.read"))
		framePerm.Post(r, "/", m.handler.Upload, framePerm.Private("Upload media", "media.write"))
		framePerm.Post(r, "/folders", m.handler.CreateFolder, framePerm.Private("Create media folder", "media.write"))
		framePerm.Put(r, "/folders/{id}", m.handler.UpdateFolder, framePerm.Private("Update media folder", "media.write"))
		framePerm.Delete(r, "/folders/{id}", m.handler.DeleteFolder, framePerm.Private("Delete media folder", "media.delete"))
		framePerm.Post(r, "/actions/bulk-move", m.handler.BulkMove, framePerm.Private("Bulk move media resources", "media.write"))
		framePerm.Post(r, "/actions/bulk-delete", m.handler.BulkDelete, framePerm.Private("Bulk delete media resources", "media.delete"))
		framePerm.Get(r, "/{id}", m.handler.GetMedia, framePerm.Private("Get media", "media.read"))
		framePerm.Delete(r, "/{id}", m.handler.DeleteMedia, framePerm.Private("Delete media", "media.delete"))
		framePerm.Put(r, "/{id}", m.handler.UpdateMedia, framePerm.Private("Update media", "media.write"))
		framePerm.Get(r, "/{id}/signed", m.handler.GetSignedURL, framePerm.Private("Get signed media URL", "media.read"))
	})
}

// Ensure MediaModule implements Module interface
var _ core.Module = (*MediaModule)(nil)
