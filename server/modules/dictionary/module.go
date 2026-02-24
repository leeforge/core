package dictionary

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type DictionaryModule struct {
	logger  logging.Logger
	handler *DictionaryHandler
}

func NewDictionaryModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize Service
	svc := NewDictionaryService(deps.Client, logger)

	// Initialize Handler
	handler := NewDictionaryHandler(svc, logger)

	return &DictionaryModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *DictionaryModule) Name() string {
	return "dictionary"
}

// RegisterPublicRoutes registers public dictionary endpoints
func (m *DictionaryModule) RegisterPublicRoutes(r chi.Router) {
	framePerm.Get(r, "/dictionaries/{code}/details", m.handler.GetDictionaryByCode, framePerm.Public("Get dictionary by code", "dictionary.read"))
}

// RegisterPrivateRoutes registers protected dictionary endpoints (admin management)
func (m *DictionaryModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/dictionaries", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.ListDictionaries, framePerm.Private("List dictionaries", "dictionary.read"))
		framePerm.Post(r, "/", m.handler.CreateDictionary, framePerm.Private("Create dictionary", "dictionary.write"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Put(r, "/", m.handler.UpdateDictionary, framePerm.Private("Update dictionary", "dictionary.write"))
			framePerm.Delete(r, "/", m.handler.DeleteDictionary, framePerm.Private("Delete dictionary", "dictionary.delete"))
			framePerm.Post(r, "/details", m.handler.CreateDetail, framePerm.Private("Create dictionary detail", "dictionary.write"))
		})
	})

	r.Route("/dictionary-details", func(r chi.Router) {
		r.Route("/{id}", func(r chi.Router) {
			framePerm.Put(r, "/", m.handler.UpdateDetail, framePerm.Private("Update dictionary detail", "dictionary.write"))
			framePerm.Delete(r, "/", m.handler.DeleteDetail, framePerm.Private("Delete dictionary detail", "dictionary.delete"))
		})
	})
}
