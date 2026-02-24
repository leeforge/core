package permission

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	entpermission "github.com/leeforge/core/server/ent/permission"
	"github.com/leeforge/core/server/permissionsync"

	"github.com/leeforge/framework/logging"
	frameperm "github.com/leeforge/framework/permission"
)

type PermissionService struct {
	client *ent.Client
	logger logging.Logger
	router chi.Routes
	syncer *permissionsync.Syncer
}

func NewPermissionService(client *ent.Client, logger logging.Logger, router chi.Routes, syncer *permissionsync.Syncer) *PermissionService {
	return &PermissionService{
		client: client,
		logger: logger,
		router: router,
		syncer: syncer,
	}
}

// ListPermissions retrieves permissions with optional filters and pagination.
func (s *PermissionService) ListPermissions(ctx context.Context, opts ListPermissionsOptions) ([]*ent.Permission, int, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 200 {
		opts.PageSize = 50
	}
	if opts.Keyword != "" {
		opts.Keyword = normalizeKeyword(opts.Keyword)
	}

	query := s.client.Permission.Query()

	if opts.Scope != "" {
		query = query.Where(entpermission.ScopeEQ(entpermission.Scope(opts.Scope)))
	}
	if opts.Status != "" {
		query = query.Where(entpermission.StatusEQ(entpermission.Status(opts.Status)))
	}
	if opts.Keyword != "" {
		query = query.Where(entpermission.Or(
			entpermission.CodeContainsFold(opts.Keyword),
			entpermission.NameContainsFold(opts.Keyword),
			entpermission.DescriptionContainsFold(opts.Keyword),
		))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	permissions, err := query.
		Order(ent.Asc(entpermission.FieldCode)).
		Offset(offset).
		Limit(opts.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return permissions, total, nil
}

func normalizeKeyword(keyword string) string {
	return strings.TrimSpace(keyword)
}

// SyncEndpoints scans all registered routes and syncs permissions to database.
func (s *PermissionService) SyncEndpoints(ctx context.Context) (*SyncEndpointsResult, error) {
	if s.router == nil || s.syncer == nil {
		return nil, fmt.Errorf("permission syncer not configured")
	}

	snapshot, err := frameperm.SnapshotFromRouter(s.router)
	if err != nil {
		return nil, fmt.Errorf("failed to build permission snapshot: %w", err)
	}

	// Permission metadata sync is a global/platform operation and should not inherit user identity.
	syncCtx := core.WithoutIdentity(core.WithoutTenant(ctx))
	syncResult, err := s.syncer.SyncSnapshot(syncCtx, snapshot, permissionsync.DefaultSyncOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to sync permissions: %w", err)
	}

	return &SyncEndpointsResult{
		TotalRoutes:   syncResult.TotalRoutes,
		Created:       syncResult.APICreated,
		Updated:       syncResult.APIUpdated,
		Deleted:       syncResult.APIDeleted,
		CreatedRoutes: syncResult.CreatedRoutes,
		UpdatedRoutes: syncResult.UpdatedRoutes,
		DeletedRoutes: syncResult.DeletedRoutes,
	}, nil
}

// GetSyncStatus gets the current sync status.
func (s *PermissionService) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	// Count API permissions
	count, err := s.client.APIPermission.Query().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count permissions: %w", err)
	}

	// Get list of all permissions
	perms, err := s.client.APIPermission.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions: %w", err)
	}

	// Group by method
	byMethod := make(map[string]int)
	for _, perm := range perms {
		byMethod[string(perm.Method)]++
	}

	return &SyncStatus{
		TotalEndpoints: count,
		ByMethod:       byMethod,
		Endpoints:      s.routesToStrings(perms),
	}, nil
}

// routesToStrings converts APIPermission slice to string slice.
func (s *PermissionService) routesToStrings(perms []*ent.APIPermission) []string {
	result := make([]string, len(perms))
	for i, perm := range perms {
		result[i] = fmt.Sprintf("%s %s", perm.Method, perm.Path)
	}
	return result
}
