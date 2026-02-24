package database

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
)

// SoftDeletable is an interface that entities with soft delete support must implement
type SoftDeletable interface {
	// SetDeletedAt sets the deleted_at timestamp
	SetDeletedAt(time.Time)
	// GetDeletedAt returns the deleted_at timestamp
	GetDeletedAt() *time.Time
}

// QueryFilter represents different query filtering modes for soft-deleted records
type QueryFilter int

const (
	// FilterExcludeDeleted excludes soft-deleted records (default behavior)
	FilterExcludeDeleted QueryFilter = iota
	// FilterIncludeDeleted includes both active and soft-deleted records
	FilterIncludeDeleted
	// FilterOnlyDeleted returns only soft-deleted records
	FilterOnlyDeleted
)

// SoftDeleteOptions contains options for soft delete operations
type SoftDeleteOptions struct {
	// Filter determines which records to include in queries
	Filter QueryFilter
	// Timestamp allows setting a custom deletion timestamp (defaults to time.Now())
	Timestamp *time.Time
}

// DefaultSoftDeleteOptions returns default options for soft delete operations
func DefaultSoftDeleteOptions() *SoftDeleteOptions {
	return &SoftDeleteOptions{
		Filter:    FilterExcludeDeleted,
		Timestamp: nil,
	}
}

// SoftDelete performs a soft delete on an entity by setting deleted_at timestamp
// This function works with any Ent entity that has a deleted_at field
//
// Example:
//
//	err := database.SoftDelete(ctx, client.User.UpdateOneID(userID), nil)
func SoftDelete(ctx context.Context, mutation any, opts *SoftDeleteOptions) error {
	if opts == nil {
		opts = DefaultSoftDeleteOptions()
	}

	deletedAt := time.Now()
	if opts.Timestamp != nil {
		deletedAt = *opts.Timestamp
	}

	// Type assertion to set deleted_at field
	switch m := mutation.(type) {
	case interface {
		SetDeletedAt(time.Time) any
		Save(context.Context) (any, error)
	}:
		m.SetDeletedAt(deletedAt)
		_, err := m.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to soft delete entity: %w", err)
		}
		return nil
	case interface{ SetDeletedAt(time.Time) }:
		m.SetDeletedAt(deletedAt)
		return nil
	default:
		return fmt.Errorf("entity does not support soft delete: missing deleted_at field")
	}
}

// SoftDeleteBulk performs a soft delete on multiple entities matching the query
//
// Example:
//
//	deleted, err := database.SoftDeleteBulk(ctx, client.User.Query().Where(user.StatusEQ("inactive")), nil)
func SoftDeleteBulk(ctx context.Context, query any, opts *SoftDeleteOptions) (int, error) {
	if opts == nil {
		opts = DefaultSoftDeleteOptions()
	}

	deletedAt := time.Now()
	if opts.Timestamp != nil {
		deletedAt = *opts.Timestamp
	}

	// Type assertion for bulk update
	type BulkUpdater interface {
		Update() any
	}

	q, ok := query.(BulkUpdater)
	if !ok {
		return 0, fmt.Errorf("query does not support bulk operations")
	}

	update := q.Update()

	// Set deleted_at on the update builder
	switch u := update.(type) {
	case interface {
		SetDeletedAt(time.Time) any
		Save(context.Context) (int, error)
	}:
		u.SetDeletedAt(deletedAt)
		affected, err := u.Save(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to bulk soft delete: %w", err)
		}
		return affected, nil
	default:
		return 0, fmt.Errorf("entity does not support soft delete: missing deleted_at field")
	}
}

// Restore restores a soft-deleted entity by clearing the deleted_at timestamp
//
// Example:
//
//	err := database.Restore(ctx, client.User.UpdateOneID(userID))
func Restore(ctx context.Context, mutation any) error {
	// Type assertion to clear deleted_at field
	switch m := mutation.(type) {
	case interface {
		ClearDeletedAt() any
		Save(context.Context) (any, error)
	}:
		m.ClearDeletedAt()
		_, err := m.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to restore entity: %w", err)
		}
		return nil
	case interface{ ClearDeletedAt() }:
		m.ClearDeletedAt()
		return nil
	default:
		return fmt.Errorf("entity does not support restore: missing deleted_at field")
	}
}

// RestoreBulk restores multiple soft-deleted entities matching the query
//
// Example:
//
//	restored, err := database.RestoreBulk(ctx, client.User.Query().Where(user.DeletedAtNotNil()))
func RestoreBulk(ctx context.Context, query any) (int, error) {
	// Type assertion for bulk update
	type BulkUpdater interface {
		Update() any
	}

	q, ok := query.(BulkUpdater)
	if !ok {
		return 0, fmt.Errorf("query does not support bulk operations")
	}

	update := q.Update()

	// Clear deleted_at on the update builder
	switch u := update.(type) {
	case interface {
		ClearDeletedAt() any
		Save(context.Context) (int, error)
	}:
		u.ClearDeletedAt()
		affected, err := u.Save(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to bulk restore: %w", err)
		}
		return affected, nil
	default:
		return 0, fmt.Errorf("entity does not support restore: missing deleted_at field")
	}
}

// ApplyFilter applies the appropriate filter to a query based on QueryFilter option
// This is a helper function to be used in repository methods
//
// Example:
//
//	query := database.ApplyFilter(client.User.Query(), database.FilterExcludeDeleted)
func ApplyFilter(query any, filter QueryFilter) any {
	type Filterable interface {
		Where(...func(*sql.Selector))
	}

	q, ok := query.(Filterable)
	if !ok {
		return query
	}

	switch filter {
	case FilterExcludeDeleted:
		// Default: exclude soft-deleted records
		q.Where(func(s *sql.Selector) {
			s.Where(sql.IsNull("deleted_at"))
		})
	case FilterOnlyDeleted:
		// Only include soft-deleted records
		q.Where(func(s *sql.Selector) {
			s.Where(sql.NotNull("deleted_at"))
		})
	case FilterIncludeDeleted:
		// Include all records (no filter)
		// Do nothing
	}

	return query
}

// WithDeleted returns a query that includes soft-deleted records
// This is a convenience wrapper around ApplyFilter
//
// Example:
//
//	query := database.WithDeleted(client.User.Query())
func WithDeleted(query any) any {
	return ApplyFilter(query, FilterIncludeDeleted)
}

// OnlyDeleted returns a query that only includes soft-deleted records
// This is a convenience wrapper around ApplyFilter
//
// Example:
//
//	query := database.OnlyDeleted(client.User.Query())
func OnlyDeleted(query any) any {
	return ApplyFilter(query, FilterOnlyDeleted)
}

// IsDeleted checks if an entity has been soft-deleted
//
// Example:
//
//	if database.IsDeleted(user) {
//	    // Handle deleted user
//	}
func IsDeleted(entity any) bool {
	type HasDeletedAt interface {
		GetDeletedAt() *time.Time
	}

	e, ok := entity.(HasDeletedAt)
	if !ok {
		return false
	}

	deletedAt := e.GetDeletedAt()
	return deletedAt != nil && !deletedAt.IsZero()
}

// ForceDelete permanently deletes an entity, bypassing soft delete
// Use with caution as this operation is irreversible
//
// Example:
//
//	err := database.ForceDelete(ctx, client.User.DeleteOneID(userID))
func ForceDelete(ctx context.Context, deletion any) error {
	type Executable interface {
		Exec(context.Context) error
	}

	d, ok := deletion.(Executable)
	if !ok {
		return fmt.Errorf("deletion does not support execution")
	}

	if err := d.Exec(ctx); err != nil {
		return fmt.Errorf("failed to force delete entity: %w", err)
	}

	return nil
}

// ForceDeleteBulk permanently deletes multiple entities matching the query
// Use with caution as this operation is irreversible
//
// Example:
//
//	deleted, err := database.ForceDeleteBulk(ctx, client.User.Delete().Where(user.DeletedAtNotNil()))
func ForceDeleteBulk(ctx context.Context, deletion any) (int, error) {
	type BulkExecutable interface {
		Exec(context.Context) (int, error)
	}

	d, ok := deletion.(BulkExecutable)
	if !ok {
		return 0, fmt.Errorf("deletion does not support bulk execution")
	}

	affected, err := d.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk force delete: %w", err)
	}

	return affected, nil
}
