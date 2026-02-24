package audit

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"

	"github.com/leeforge/framework/logging"
)

// Event describes one audit event payload.
type Event struct {
	Action     string
	Resource   string
	ResourceID string
	Before     map[string]any
	After      map[string]any
	IPAddress  string
	UserAgent  string
}

// Service persists audit events in best-effort mode.
type Service struct {
	client *ent.Client
	logger logging.Logger
}

// NewService creates an audit service.
func NewService(client *ent.Client, logger logging.Logger) *Service {
	return &Service{
		client: client,
		logger: logger,
	}
}

// Record writes an event using identity from context.
func (s *Service) Record(ctx context.Context, event Event) error {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		return nil
	}
	tenantID, _ := core.GetTenantID(ctx)
	return s.RecordWithActor(ctx, identity.UserID, tenantID, event)
}

// RecordWithActor writes an event for specified actor and domain scope.
func (s *Service) RecordWithActor(ctx context.Context, actorID uuid.UUID, tenantID string, event Event) error {
	if s == nil || s.client == nil {
		return nil
	}
	action := strings.TrimSpace(event.Action)
	resource := strings.TrimSpace(event.Resource)
	if action == "" || resource == "" {
		return nil
	}

	// Resolve owner_domain_id: prefer domain ID from context, fallback to parsing tenantID as UUID.
	var domainID *uuid.UUID
	if did, ok := core.GetDomainID(ctx); ok {
		if parsed, err := uuid.Parse(did); err == nil {
			domainID = &parsed
		}
	}
	if domainID == nil {
		scopedTenantID := strings.TrimSpace(tenantID)
		if scopedTenantID != "" {
			if parsed, err := uuid.Parse(scopedTenantID); err == nil {
				domainID = &parsed
			}
		}
	}

	builder := s.client.AuditLog.Create().
		SetNillableOwnerDomainID(domainID).
		SetAction(action).
		SetResource(resource).
		SetBefore(nonNilMap(event.Before)).
		SetAfter(nonNilMap(event.After)).
		SetIPAddress(strings.TrimSpace(event.IPAddress)).
		SetUserAgent(strings.TrimSpace(event.UserAgent)).
		AddUserIDs(actorID)

	if rid := strings.TrimSpace(event.ResourceID); rid != "" {
		builder.SetResourceID(rid)
	}

	_, err := builder.Save(ctx)
	return err
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
