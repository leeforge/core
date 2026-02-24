package menu

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/menu"
	"github.com/leeforge/core/server/ent/role"
	"github.com/leeforge/core/server/pkg/errors"

	"github.com/leeforge/framework/logging"
)

// MenuNode represents a menu in a tree structure
type MenuNode struct {
	*ent.Menu
	Children []*MenuNode `json:"children,omitempty"`
}

type MenuService struct {
	client *ent.Client
	logger logging.Logger
}

func NewMenuService(client *ent.Client, logger logging.Logger) *MenuService {
	return &MenuService{
		client: client,
		logger: logger,
	}
}

// GetMenuTree returns the menu tree based on user roles from context.
// Super admin roles see all menus, others see only menus assigned to their roles.
func (s *MenuService) GetMenuTree(ctx context.Context, roleCodes []string, isSuperAdmin bool) ([]*MenuNode, error) {
	var menus []*ent.Menu
	var err error

	if isSuperAdmin {
		// Super admin sees all menus
		menus, err = s.client.Menu.Query().
			Order(ent.Asc(menu.FieldSort)).
			All(ctx)
	} else {
		// Regular users see menus assigned to their roles
		menus, err = s.client.Menu.Query().
			Where(menu.HasRolesWith(role.CodeIn(roleCodes...))).
			Order(ent.Asc(menu.FieldSort)).
			All(ctx)
		if err == nil {
			// Ensure parent menus are included for valid tree structure
			menus, err = s.ensureParentMenus(ctx, menus)
		}
	}

	if err != nil {
		return nil, errors.NewInternalError("Failed to query menus", err)
	}

	return s.buildTree(menus, uuid.Nil), nil
}

// ensureParentMenus adds missing parent menus to maintain tree structure
func (s *MenuService) ensureParentMenus(ctx context.Context, menus []*ent.Menu) ([]*ent.Menu, error) {
	menuMap := make(map[uuid.UUID]*ent.Menu)
	var missingParentIDs []uuid.UUID

	// Index existing menus
	for _, m := range menus {
		menuMap[m.ID] = m
	}

	// Find missing parents
	for _, m := range menus {
		if m.ParentID != uuid.Nil {
			if _, exists := menuMap[m.ParentID]; !exists {
				missingParentIDs = append(missingParentIDs, m.ParentID)
			}
		}
	}

	// Recursively fetch missing parents
	for len(missingParentIDs) > 0 {
		parents, err := s.client.Menu.Query().
			Where(menu.IDIn(missingParentIDs...)).
			All(ctx)
		if err != nil {
			return nil, err
		}

		missingParentIDs = nil
		for _, p := range parents {
			if _, exists := menuMap[p.ID]; !exists {
				menuMap[p.ID] = p
				menus = append(menus, p)
				// Check if this parent also needs its parent
				if p.ParentID != uuid.Nil {
					if _, exists := menuMap[p.ParentID]; !exists {
						missingParentIDs = append(missingParentIDs, p.ParentID)
					}
				}
			}
		}
	}

	return menus, nil
}

// CreateMenu creates a new menu item
func (s *MenuService) CreateMenu(ctx context.Context, input *ent.Menu) (*ent.Menu, error) {
	builder := s.client.Menu.Create().
		SetPath(input.Path).
		SetName(input.Name).
		SetComponent(input.Component).
		SetRedirect(input.Redirect).
		SetHidden(input.Hidden).
		SetAffix(input.Affix).
		SetIcon(input.Icon).
		SetSort(input.Sort).
		SetMeta(input.Meta).
		SetParams(input.Params)

	if input.ParentID != uuid.Nil {
		builder.SetParentID(input.ParentID)
	}

	m, err := builder.Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create menu", err)
	}
	return m, nil
}

// UpdateMenu updates an existing menu
func (s *MenuService) UpdateMenu(ctx context.Context, id uuid.UUID, input *ent.Menu) (*ent.Menu, error) {
	m, err := s.client.Menu.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Menu not found", err)
		}
		return nil, errors.NewInternalError("Failed to query menu", err)
	}

	updater := s.client.Menu.UpdateOne(m).
		SetPath(input.Path).
		SetName(input.Name).
		SetComponent(input.Component).
		SetRedirect(input.Redirect).
		SetHidden(input.Hidden).
		SetAffix(input.Affix).
		SetIcon(input.Icon).
		SetSort(input.Sort).
		SetMeta(input.Meta).
		SetParams(input.Params)

	if input.ParentID != uuid.Nil {
		updater.SetParentID(input.ParentID)
	} else {
		updater.ClearParent()
	}

	updated, err := updater.Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to update menu", err)
	}
	return updated, nil
}

// DeleteMenu deletes a menu and recursively deletes its children
func (s *MenuService) DeleteMenu(ctx context.Context, id uuid.UUID) error {
	// Start transaction
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to start transaction", err)
	}

	if err := s.recursiveDelete(ctx, tx, id); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.NewInternalError("Failed to commit transaction", err)
	}
	return nil
}

func (s *MenuService) recursiveDelete(ctx context.Context, tx *ent.Tx, id uuid.UUID) error {
	// Find children first
	children, err := tx.Menu.Query().
		Where(menu.ParentID(id)).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to query children: %w", err)
	}

	// Delete children recursively
	for _, childID := range children {
		if err := s.recursiveDelete(ctx, tx, childID); err != nil {
			return err
		}
	}

	// Delete the menu itself
	if err := tx.Menu.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete menu %s: %w", id, err)
	}

	return nil
}

// GetMenuByID retrieves a single menu by ID
func (s *MenuService) GetMenuByID(ctx context.Context, id uuid.UUID) (*ent.Menu, error) {
	m, err := s.client.Menu.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Menu not found", err)
		}
		return nil, errors.NewInternalError("Failed to query menu", err)
	}
	return m, nil
}

// BatchCreateMenus creates multiple menus in a single transaction.
// Supports parent-child relationships within the same batch via tempId references.
// Each input menu can have a TempID field and a TempParentID field:
// - TempID: a client-assigned identifier for referencing within this batch
// - TempParentID: references another menu's TempID in this batch as its parent
func (s *MenuService) BatchCreateMenus(ctx context.Context, inputs []BatchMenuInput) ([]*ent.Menu, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to start transaction", err)
	}

	// Phase 1: Create menus without intra-batch parent references first
	tempIDMap := make(map[string]uuid.UUID) // tempID -> real UUID
	var results []*ent.Menu
	var deferred []BatchMenuInput

	for _, input := range inputs {
		if input.TempParentID != "" {
			// This menu references another menu in this batch
			deferred = append(deferred, input)
			continue
		}
		m, err := s.createMenuInTx(ctx, tx, &input)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if input.TempID != "" {
			tempIDMap[input.TempID] = m.ID
		}
		results = append(results, m)
	}

	// Phase 2: Create deferred menus (those referencing batch siblings)
	for _, input := range deferred {
		realParentID, ok := tempIDMap[input.TempParentID]
		if !ok {
			tx.Rollback()
			return nil, fmt.Errorf("tempParentId '%s' not found in batch", input.TempParentID)
		}
		input.ParentID = &realParentID
		m, err := s.createMenuInTx(ctx, tx, &input)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if input.TempID != "" {
			tempIDMap[input.TempID] = m.ID
		}
		results = append(results, m)
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit transaction", err)
	}
	return results, nil
}

// createMenuInTx creates a single menu within a transaction
func (s *MenuService) createMenuInTx(ctx context.Context, tx *ent.Tx, input *BatchMenuInput) (*ent.Menu, error) {
	builder := tx.Menu.Create().
		SetPath(input.Path).
		SetName(input.Name).
		SetComponent(input.Component).
		SetRedirect(input.Redirect).
		SetHidden(input.Hidden).
		SetAffix(input.Affix).
		SetIcon(input.Icon).
		SetSort(input.Sort).
		SetMeta(input.Meta).
		SetParams(input.Params)

	if input.ParentID != nil && *input.ParentID != uuid.Nil {
		builder.SetParentID(*input.ParentID)
	}

	m, err := builder.Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Sprintf("Failed to create menu '%s'", input.Name), err)
	}
	return m, nil
}

// GetAllMenus retrieves all menus ordered by sort field
func (s *MenuService) GetAllMenus(ctx context.Context) ([]*ent.Menu, error) {
	menus, err := s.client.Menu.Query().
		Order(ent.Asc(menu.FieldSort)).
		All(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to query menus", err)
	}
	return menus, nil
}

// buildTree recursively builds the menu tree from a flat list
func (s *MenuService) buildTree(menus []*ent.Menu, parentID uuid.UUID) []*MenuNode {
	var nodes []*MenuNode
	for _, m := range menus {
		// Nil check for parent_id based on current logic (uuid.Nil for root)
		isMatch := false
		if parentID == uuid.Nil {
			isMatch = m.ParentID == uuid.Nil || m.ParentID.String() == "00000000-0000-0000-0000-000000000000"
		} else {
			isMatch = m.ParentID == parentID
		}

		if isMatch {
			node := &MenuNode{
				Menu: m,
			}
			node.Children = s.buildTree(menus, m.ID)
			nodes = append(nodes, node)
		}
	}

	// Re-sort by Sort field just in case
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Sort < nodes[j].Sort
	})

	return nodes
}
