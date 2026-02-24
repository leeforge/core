package operationlog

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	entops "github.com/leeforge/core/server/ent/operationlog"
	"github.com/leeforge/core/server/ent/predicate"
	"github.com/leeforge/core/server/pkg/errors"
	"github.com/leeforge/framework/logging"
)

// CreateLogInput 创建日志输入参数
type CreateLogInput struct {
	RequestID     string
	Method        string
	Path          string
	QueryParams   string
	Body          string
	Status        int
	Latency       int64
	ClientIP      string
	UserAgent     string
	ErrorMessage  string
	UserID        *uuid.UUID
	OwnerDomainID *uuid.UUID
}

// LogQueryInput 查询参数
type LogQueryInput struct {
	Page      int
	PageSize  int
	StartDate *time.Time
	EndDate   *time.Time
	Method    string
	Path      string
	Status    *int
	UserID    *uuid.UUID
	ClientIP  string
	RequestID string
}

type OperationLogService struct {
	client *ent.Client
	logger logging.Logger
}

func NewOperationLogService(client *ent.Client, logger logging.Logger) *OperationLogService {
	return &OperationLogService{
		client: client,
		logger: logger,
	}
}

// Create Asynchronously creates an operation log
// We use a separate context or background context to ensure log is saved even if request context is cancelled
func (s *OperationLogService) Create(input *CreateLogInput) {
	// Use a background context with timeout for the async operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	builder := s.client.OperationLog.Create().
		SetMethod(input.Method).
		SetPath(input.Path).
		SetStatus(input.Status).
		SetLatency(input.Latency).
		SetClientIP(input.ClientIP)

	if input.RequestID != "" {
		builder.SetRequestID(input.RequestID)
	}
	if input.QueryParams != "" {
		builder.SetQueryParams(input.QueryParams)
	}
	if input.Body != "" {
		builder.SetBody(input.Body)
	}
	if input.UserAgent != "" {
		builder.SetUserAgent(input.UserAgent)
	}
	if input.ErrorMessage != "" {
		builder.SetErrorMessage(input.ErrorMessage)
	}
	if input.UserID != nil {
		builder.SetUserID(*input.UserID)
	}
	if input.OwnerDomainID != nil {
		builder.SetOwnerDomainID(*input.OwnerDomainID)
	}

	if _, err := builder.Save(ctx); err != nil {
		s.logger.Error("Failed to save operation log",
			zap.Error(err),
			zap.String("path", input.Path),
			zap.String("method", input.Method),
		)
	}
}

// List queries operation logs with filters
func (s *OperationLogService) List(ctx context.Context, input *LogQueryInput) ([]*ent.OperationLog, int, error) {
	query := s.client.OperationLog.Query()

	// Apply filters
	var predicates []predicate.OperationLog

	if input.StartDate != nil {
		predicates = append(predicates, entops.CreatedAtGTE(*input.StartDate))
	}
	if input.EndDate != nil {
		predicates = append(predicates, entops.CreatedAtLTE(*input.EndDate))
	}
	if input.Method != "" {
		predicates = append(predicates, entops.MethodEQ(input.Method))
	}
	if input.Path != "" {
		predicates = append(predicates, entops.PathContains(input.Path))
	}
	if input.Status != nil {
		predicates = append(predicates, entops.StatusEQ(*input.Status))
	}
	if input.UserID != nil {
		predicates = append(predicates, entops.UserID(*input.UserID))
	}
	if input.ClientIP != "" {
		predicates = append(predicates, entops.ClientIPEQ(input.ClientIP))
	}
	if input.RequestID != "" {
		predicates = append(predicates, entops.RequestIDEQ(input.RequestID))
	}

	if len(predicates) > 0 {
		query.Where(predicates...)
	}

	// Count total
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to count logs", err)
	}

	// Pagination & Order
	logs, err := query.
		Order(ent.Desc(entops.FieldCreatedAt)).
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		All(ctx)

	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to query logs", err)
	}

	return logs, total, nil
}

// ClearOldLogs deletes logs older than retention days
func (s *OperationLogService) ClearOldLogs(ctx context.Context, retentionDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	count, err := s.client.OperationLog.Delete().
		Where(entops.CreatedAtLT(cutoff)).
		Exec(ctx)

	if err != nil {
		return 0, errors.NewInternalError("Failed to clear old logs", err)
	}

	return count, nil
}

// Delete deletes a specific log entry
func (s *OperationLogService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.client.OperationLog.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Log not found", err)
		}
		return errors.NewInternalError("Failed to delete log", err)
	}
	return nil
}
