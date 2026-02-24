package dictionary

import (
	"context"

	"github.com/google/uuid"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/dictionary"
	"github.com/leeforge/core/server/ent/dictionarydetail"
	"github.com/leeforge/core/server/pkg/errors"

	"github.com/leeforge/framework/logging"
)

type DictionaryService struct {
	client *ent.Client
	logger logging.Logger
	// TODO: Add cache support
}

func NewDictionaryService(client *ent.Client, logger logging.Logger) *DictionaryService {
	return &DictionaryService{
		client: client,
		logger: logger,
	}
}

// ListDictionaries returns all dictionaries
func (s *DictionaryService) ListDictionaries(ctx context.Context) ([]*ent.Dictionary, error) {
	dicts, err := s.client.Dictionary.Query().
		Order(ent.Asc(dictionary.FieldCode)).
		All(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to query dictionaries", err)
	}
	return dicts, nil
}

// CreateDictionary creates a new dictionary
func (s *DictionaryService) CreateDictionary(ctx context.Context, input *ent.Dictionary) (*ent.Dictionary, error) {
	d, err := s.client.Dictionary.Create().
		SetCode(input.Code).
		SetName(input.Name).
		SetDescription(input.Description).
		SetStatus(input.Status).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Dictionary code already exists", err)
		}
		return nil, errors.NewInternalError("Failed to create dictionary", err)
	}
	return d, nil
}

// UpdateDictionary updates a dictionary
func (s *DictionaryService) UpdateDictionary(ctx context.Context, id uuid.UUID, input *ent.Dictionary) (*ent.Dictionary, error) {
	d, err := s.client.Dictionary.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Dictionary not found", err)
		}
		return nil, errors.NewInternalError("Failed to query dictionary", err)
	}

	updater := s.client.Dictionary.UpdateOne(d).
		SetName(input.Name).
		SetDescription(input.Description).
		SetStatus(input.Status)

	// Code is usually immutable or handled carefully
	if input.Code != "" {
		updater.SetCode(input.Code)
	}

	updated, err := updater.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Dictionary code already exists", err)
		}
		return nil, errors.NewInternalError("Failed to update dictionary", err)
	}
	return updated, nil
}

// DeleteDictionary deletes a dictionary and its items
func (s *DictionaryService) DeleteDictionary(ctx context.Context, id uuid.UUID) error {
	// Delete associated items
	_, err := s.client.DictionaryDetail.Delete().
		Where(dictionarydetail.HasDictionaryWith(dictionary.ID(id))).
		Exec(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to delete dictionary items", err)
	}

	// Delete dictionary
	if err := s.client.Dictionary.DeleteOneID(id).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Dictionary not found", err)
		}
		return errors.NewInternalError("Failed to delete dictionary", err)
	}

	return nil
}

// GetDetailsByCode returns dictionary details by dictionary code
func (s *DictionaryService) GetDetailsByCode(ctx context.Context, code string) ([]*ent.DictionaryDetail, error) {
	// TODO: Check cache first

	d, err := s.client.Dictionary.Query().
		Where(dictionary.Code(code)).
		WithItems(func(q *ent.DictionaryDetailQuery) {
			q.Order(ent.Asc(dictionarydetail.FieldSort))
			q.Where(dictionarydetail.Status(true))
		}).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Dictionary code not found", err)
		}
		return nil, errors.NewInternalError("Failed to query dictionary", err)
	}

	return d.Edges.Items, nil
}

// CreateDetail creates a new dictionary item
func (s *DictionaryService) CreateDetail(ctx context.Context, dictID uuid.UUID, input *ent.DictionaryDetail) (*ent.DictionaryDetail, error) {
	item, err := s.client.DictionaryDetail.Create().
		SetDictionaryID(dictID).
		SetLabel(input.Label).
		SetValue(input.Value).
		SetExtend(input.Extend).
		SetSort(input.Sort).
		SetStatus(input.Status).
		Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create dictionary item", err)
	}
	return item, nil
}

// UpdateDetail updates a dictionary item
func (s *DictionaryService) UpdateDetail(ctx context.Context, id uuid.UUID, input *ent.DictionaryDetail) (*ent.DictionaryDetail, error) {
	item, err := s.client.DictionaryDetail.UpdateOneID(id).
		SetLabel(input.Label).
		SetValue(input.Value).
		SetExtend(input.Extend).
		SetSort(input.Sort).
		SetStatus(input.Status).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Dictionary item not found", err)
		}
		return nil, errors.NewInternalError("Failed to update dictionary item", err)
	}
	return item, nil
}

// DeleteDetail deletes a dictionary item
func (s *DictionaryService) DeleteDetail(ctx context.Context, id uuid.UUID) error {
	if err := s.client.DictionaryDetail.DeleteOneID(id).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Dictionary item not found", err)
		}
		return errors.NewInternalError("Failed to delete dictionary item", err)
	}
	return nil
}
