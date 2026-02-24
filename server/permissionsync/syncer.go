package permissionsync

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/leeforge/framework/logging"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/apipermission"
	"github.com/leeforge/core/server/ent/permission"

	frameperm "github.com/leeforge/framework/permission"
)

// SyncOptions controls sync behavior.
type SyncOptions struct {
	MarkDeprecated   bool
	DeleteOrphanAPIs bool
	DeleteOrphanMaps bool
}

// SyncResult summarizes a sync run.
type SyncResult struct {
	TotalRoutes           int
	PermissionsCreated    int
	PermissionsUpdated    int
	PermissionsDeprecated int
	APICreated            int
	APIUpdated            int
	APIDeleted            int
	MappingCreated        int
	MappingDeleted        int
	CreatedRoutes         []string
	UpdatedRoutes         []string
	DeletedRoutes         []string
}

// Syncer syncs permission snapshots into the database.
type Syncer struct {
	client *ent.Client
	logger logging.Logger
}

// NewSyncer creates a new permission syncer.
func NewSyncer(client *ent.Client, logger logging.Logger) *Syncer {
	return &Syncer{
		client: client,
		logger: logger,
	}
}

// DefaultSyncOptions returns the default sync options.
func DefaultSyncOptions() SyncOptions {
	return SyncOptions{
		MarkDeprecated:   true,
		DeleteOrphanAPIs: true,
		DeleteOrphanMaps: true,
	}
}

// SyncSnapshot syncs a permission snapshot to the database.
func (s *Syncer) SyncSnapshot(ctx context.Context, snapshot frameperm.Snapshot, opts SyncOptions) (*SyncResult, error) {
	result := &SyncResult{TotalRoutes: len(snapshot.Routes)}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	apiMap := make(map[string]*ent.APIPermission)

	permRes, err := s.syncPermissions(ctx, tx, snapshot.Permissions, opts)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	result.PermissionsCreated = permRes.created
	result.PermissionsUpdated = permRes.updated
	result.PermissionsDeprecated = permRes.deprecated

	apiRes, err := s.syncAPIs(ctx, tx, snapshot.Routes, opts)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	result.APICreated = apiRes.created
	result.APIUpdated = apiRes.updated
	result.APIDeleted = apiRes.deleted
	result.CreatedRoutes = apiRes.createdRoutes
	result.UpdatedRoutes = apiRes.updatedRoutes
	result.DeletedRoutes = apiRes.deletedRoutes
	apiMap = apiRes.byKey

	mapRes, err := s.syncMappings(ctx, tx, snapshot.Mappings, apiMap, opts)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	result.MappingCreated = mapRes.created
	result.MappingDeleted = mapRes.deleted

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Permission sync completed",
		zap.Int("permissions_created", result.PermissionsCreated),
		zap.Int("permissions_updated", result.PermissionsUpdated),
		zap.Int("permissions_deprecated", result.PermissionsDeprecated),
		zap.Int("api_created", result.APICreated),
		zap.Int("api_updated", result.APIUpdated),
		zap.Int("api_deleted", result.APIDeleted),
		zap.Int("mapping_created", result.MappingCreated),
		zap.Int("mapping_deleted", result.MappingDeleted),
	)

	return result, nil
}

type permSyncResult struct {
	created    int
	updated    int
	deprecated int
}

type apiSyncResult struct {
	created       int
	updated       int
	deleted       int
	createdRoutes []string
	updatedRoutes []string
	deletedRoutes []string
	byKey         map[string]*ent.APIPermission
}

type mapSyncResult struct {
	created int
	deleted int
}

func (s *Syncer) syncPermissions(ctx context.Context, tx *ent.Tx, perms []frameperm.Permission, opts SyncOptions) (permSyncResult, error) {
	result := permSyncResult{}

	desired := make(map[string]frameperm.Permission)
	for _, perm := range perms {
		code := strings.TrimSpace(perm.Code)
		if code == "" {
			continue
		}
		desired[code] = normalizePermission(perm)
	}

	codes := make([]string, 0, len(desired))
	for code := range desired {
		codes = append(codes, code)
	}

	existing := make(map[string]*ent.Permission)
	if len(codes) > 0 {
		items, err := tx.Permission.Query().Where(permission.CodeIn(codes...)).All(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to query permissions: %w", err)
		}
		for _, item := range items {
			existing[item.Code] = item
		}
	}

	for code, perm := range desired {
		if current, ok := existing[code]; ok {
			if needsPermissionUpdate(current, perm) {
				if _, err := tx.Permission.UpdateOneID(current.ID).
					SetName(perm.Name).
					SetDescription(perm.Description).
					SetScope(permission.Scope(perm.Scope)).
					SetStatus(permission.Status(perm.Status)).
					Save(ctx); err != nil {
					return result, fmt.Errorf("failed to update permission %s: %w", code, err)
				}
				result.updated++
			}
			continue
		}

		if _, err := tx.Permission.Create().
			SetCode(code).
			SetName(perm.Name).
			SetDescription(perm.Description).
			SetScope(permission.Scope(perm.Scope)).
			SetStatus(permission.Status(perm.Status)).
			Save(ctx); err != nil {
			return result, fmt.Errorf("failed to create permission %s: %w", code, err)
		}
		result.created++
	}

	if opts.MarkDeprecated {
		items, err := tx.Permission.Query().All(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to query permissions for deprecate: %w", err)
		}
		for _, item := range items {
			if item.Scope != permission.Scope(frameperm.ScopeAPI) {
				continue
			}
			if _, ok := desired[item.Code]; ok {
				continue
			}
			if item.Status == permission.Status(frameperm.StatusDeprecated) {
				continue
			}
			if _, err := tx.Permission.UpdateOneID(item.ID).
				SetStatus(permission.Status(frameperm.StatusDeprecated)).
				Save(ctx); err != nil {
				return result, fmt.Errorf("failed to deprecate permission %s: %w", item.Code, err)
			}
			result.deprecated++
		}
	}

	return result, nil
}

func (s *Syncer) syncAPIs(ctx context.Context, tx *ent.Tx, routes []frameperm.RouteInfo, opts SyncOptions) (apiSyncResult, error) {
	result := apiSyncResult{byKey: make(map[string]*ent.APIPermission)}

	desired := make(map[string]frameperm.RouteInfo)
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		path := strings.TrimSpace(route.Path)
		if method == "" || path == "" {
			continue
		}
		if !isSupportedMethod(method) {
			s.logger.Warn("Skipping unsupported method for API permission sync",
				zap.String("method", method),
				zap.String("path", path),
			)
			continue
		}
		route.Method = method
		route.Path = path
		key := routeKey(method, path)
		desired[key] = route
	}

	existingItems, err := tx.APIPermission.Query().All(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to query API permissions: %w", err)
	}

	existing := make(map[string]*ent.APIPermission)
	for _, item := range existingItems {
		key := routeKey(string(item.Method), item.Path)
		existing[key] = item
	}

	for key, route := range desired {
		if current, ok := existing[key]; ok {
			if current.Description != route.Description || current.IsPublic != route.IsPublic {
				updated, err := tx.APIPermission.UpdateOneID(current.ID).
					SetDescription(route.Description).
					SetIsPublic(route.IsPublic).
					Save(ctx)
				if err != nil {
					return result, fmt.Errorf("failed to update api permission %s %s: %w", route.Method, route.Path, err)
				}
				result.updated++
				result.updatedRoutes = append(result.updatedRoutes, formatRoute(route.Method, route.Path))
				result.byKey[key] = updated
			} else {
				result.byKey[key] = current
			}
			delete(existing, key)
			continue
		}

		created, err := tx.APIPermission.Create().
			SetPath(route.Path).
			SetMethod(apipermission.Method(route.Method)).
			SetDescription(route.Description).
			SetIsPublic(route.IsPublic).
			Save(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to create api permission %s %s: %w", route.Method, route.Path, err)
		}
		result.created++
		result.createdRoutes = append(result.createdRoutes, formatRoute(route.Method, route.Path))
		result.byKey[key] = created
	}

	if opts.DeleteOrphanAPIs {
		for _, item := range existing {
			if err := tx.APIPermission.DeleteOneID(item.ID).Exec(ctx); err != nil {
				return result, fmt.Errorf("failed to delete api permission %s %s: %w", item.Method, item.Path, err)
			}
			result.deleted++
			result.deletedRoutes = append(result.deletedRoutes, formatRoute(string(item.Method), item.Path))
		}
	}

	sort.Strings(result.createdRoutes)
	sort.Strings(result.updatedRoutes)
	sort.Strings(result.deletedRoutes)

	return result, nil
}

func (s *Syncer) syncMappings(ctx context.Context, tx *ent.Tx, mappings []frameperm.Mapping, apiMap map[string]*ent.APIPermission, opts SyncOptions) (mapSyncResult, error) {
	result := mapSyncResult{}

	desired := make(map[string]struct{})
	for _, mapping := range mappings {
		method := strings.ToUpper(strings.TrimSpace(mapping.Method))
		path := strings.TrimSpace(mapping.Path)
		code := strings.TrimSpace(mapping.PermissionCode)
		if method == "" || path == "" || code == "" {
			continue
		}
		if !isSupportedMethod(method) {
			continue
		}
		key := routeKey(method, path)
		api, ok := apiMap[key]
		if !ok {
			s.logger.Warn("Skipping mapping for missing API",
				zap.String("method", method),
				zap.String("path", path),
				zap.String("permission", code),
			)
			continue
		}
		desired[mapKey(api.ID, code)] = struct{}{}
	}

	existingItems, err := tx.APIPermissionMap.Query().All(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to query api permission maps: %w", err)
	}
	current := make(map[string]*ent.APIPermissionMap)
	for _, item := range existingItems {
		current[mapKey(item.APIID, item.PermissionCode)] = item
	}

	for key := range desired {
		if _, ok := current[key]; ok {
			delete(current, key)
			continue
		}
		apiID, code, ok := splitMapKey(key)
		if !ok {
			continue
		}
		if _, err := tx.APIPermissionMap.Create().
			SetAPIID(apiID).
			SetPermissionCode(code).
			Save(ctx); err != nil {
			return result, fmt.Errorf("failed to create api permission map: %w", err)
		}
		result.created++
	}

	if opts.DeleteOrphanMaps {
		for _, item := range current {
			if err := tx.APIPermissionMap.DeleteOneID(item.ID).Exec(ctx); err != nil {
				return result, fmt.Errorf("failed to delete api permission map: %w", err)
			}
			result.deleted++
		}
	}

	return result, nil
}

func normalizePermission(perm frameperm.Permission) frameperm.Permission {
	if strings.TrimSpace(perm.Code) == "" {
		return perm
	}

	if strings.TrimSpace(perm.Name) == "" {
		perm.Name = perm.Code
	}
	if perm.Scope == "" {
		perm.Scope = frameperm.ScopeAPI
	}
	if perm.Status == "" {
		perm.Status = frameperm.StatusActive
	}
	return perm
}

func needsPermissionUpdate(current *ent.Permission, desired frameperm.Permission) bool {
	if current.Name != desired.Name {
		return true
	}
	if current.Description != desired.Description {
		return true
	}
	if current.Scope != permission.Scope(desired.Scope) {
		return true
	}
	if current.Status != permission.Status(desired.Status) {
		return true
	}
	return false
}

func isSupportedMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS":
		return true
	default:
		return false
	}
}

func routeKey(method, path string) string {
	return method + ":" + path
}

func mapKey(apiID uuid.UUID, code string) string {
	return apiID.String() + ":" + code
}

func splitMapKey(key string) (uuid.UUID, string, bool) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return uuid.Nil, "", false
	}
	apiID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, "", false
	}
	return apiID, parts[1], true
}

func formatRoute(method, path string) string {
	return method + " " + path
}
