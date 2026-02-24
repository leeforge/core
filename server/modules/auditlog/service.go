package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/leeforge/core/server/ent"
	entaudit "github.com/leeforge/core/server/ent/auditlog"
	"github.com/leeforge/core/server/ent/predicate"
	entuser "github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/errors"
	"github.com/leeforge/framework/logging"
)

// LogQueryInput defines audit log query parameters.
type LogQueryInput struct {
	Page       int
	PageSize   int
	StartDate  *time.Time
	EndDate    *time.Time
	Action     string
	Resource   string
	ResourceID string
	UserID     *uuid.UUID
	IPAddress  string
}

type AuditLogService struct {
	client *ent.Client
	logger logging.Logger
}

func NewAuditLogService(client *ent.Client, logger logging.Logger) *AuditLogService {
	return &AuditLogService{
		client: client,
		logger: logger,
	}
}

// List queries audit logs with filters.
func (s *AuditLogService) List(ctx context.Context, input *LogQueryInput) ([]*ent.AuditLog, int, error) {
	query := s.client.AuditLog.Query()

	var predicates []predicate.AuditLog
	if input.StartDate != nil {
		predicates = append(predicates, entaudit.CreatedAtGTE(*input.StartDate))
	}
	if input.EndDate != nil {
		predicates = append(predicates, entaudit.CreatedAtLTE(*input.EndDate))
	}
	if input.Action != "" {
		predicates = append(predicates, entaudit.ActionContainsFold(input.Action))
	}
	if input.Resource != "" {
		predicates = append(predicates, entaudit.ResourceContainsFold(input.Resource))
	}
	if input.ResourceID != "" {
		predicates = append(predicates, entaudit.ResourceIDEQ(input.ResourceID))
	}
	if input.UserID != nil {
		predicates = append(predicates, entaudit.HasUserWith(entuser.IDEQ(*input.UserID)))
	}
	if input.IPAddress != "" {
		predicates = append(predicates, entaudit.IPAddressContainsFold(input.IPAddress))
	}

	if len(predicates) > 0 {
		query.Where(predicates...)
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to count audit logs", err)
	}

	logs, err := query.
		Order(ent.Desc(entaudit.FieldCreatedAt)).
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to query audit logs", err)
	}

	return logs, total, nil
}

// ClearOldLogs deletes audit logs older than retention days.
func (s *AuditLogService) ClearOldLogs(ctx context.Context, retentionDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	count, err := s.client.AuditLog.Delete().
		Where(entaudit.CreatedAtLT(cutoff)).
		Exec(ctx)
	if err != nil {
		return 0, errors.NewInternalError("Failed to clear audit logs", err)
	}
	return count, nil
}

// Delete deletes a specific audit log entry.
func (s *AuditLogService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.client.AuditLog.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Audit log not found", err)
		}
		return errors.NewInternalError("Failed to delete audit log", err)
	}
	return nil
}
