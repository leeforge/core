package systemerror

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/predicate"
	entsys "github.com/leeforge/core/server/ent/systemerror"
	"github.com/leeforge/core/server/pkg/errors"
	"github.com/leeforge/framework/logging"
)

type CreateErrorInput struct {
	ErrorLevel    string
	ErrorType     string
	ErrorMsg      string
	ErrorStack    string
	ErrorCode     string
	RequestID     string
	RequestMethod string
	RequestPath   string
	RequestBody   string
	UserID        *uuid.UUID
}

type ErrorQueryInput struct {
	Page      int
	PageSize  int
	StartDate *time.Time
	EndDate   *time.Time
	Level     string
	Type      string
	RequestID string
}

type SystemErrorService struct {
	client *ent.Client
	logger logging.Logger
}

func NewSystemErrorService(client *ent.Client, logger logging.Logger) *SystemErrorService {
	return &SystemErrorService{
		client: client,
		logger: logger,
	}
}

// Create Asynchronously records a system error
func (s *SystemErrorService) Create(input *CreateErrorInput) {
	// Use background context to ensure log is saved even if request context is cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Default level if not provided
	level := entsys.ErrorLevelError
	if input.ErrorLevel != "" {
		level = entsys.ErrorLevel(input.ErrorLevel)
	}

	builder := s.client.SystemError.Create().
		SetErrorLevel(level).
		SetErrorType(input.ErrorType).
		SetErrorMsg(input.ErrorMsg).
		SetErrorStack(input.ErrorStack)

	if input.ErrorCode != "" {
		builder.SetErrorCode(input.ErrorCode)
	}
	if input.RequestID != "" {
		builder.SetRequestID(input.RequestID)
	}
	if input.RequestMethod != "" {
		builder.SetRequestMethod(input.RequestMethod)
	}
	if input.RequestPath != "" {
		builder.SetRequestPath(input.RequestPath)
	}
	if input.RequestBody != "" {
		builder.SetRequestBody(input.RequestBody)
	}
	if input.UserID != nil {
		builder.SetUserID(*input.UserID)
	}

	if _, err := builder.Save(ctx); err != nil {
		// Fallback to zap logger if DB write fails
		s.logger.Error("Failed to save system error log",
			zap.Error(err),
			zap.String("original_error", input.ErrorMsg),
		)
	}
}

// List queries system errors
func (s *SystemErrorService) List(ctx context.Context, input *ErrorQueryInput) ([]*ent.SystemError, int, error) {
	query := s.client.SystemError.Query()

	// Apply filters
	var predicates []predicate.SystemError

	if input.StartDate != nil {
		predicates = append(predicates, entsys.CreatedAtGTE(*input.StartDate))
	}
	if input.EndDate != nil {
		predicates = append(predicates, entsys.CreatedAtLTE(*input.EndDate))
	}
	if input.Level != "" {
		predicates = append(predicates, entsys.ErrorLevelEQ(entsys.ErrorLevel(input.Level)))
	}
	if input.Type != "" {
		predicates = append(predicates, entsys.ErrorTypeEQ(input.Type))
	}
	if input.RequestID != "" {
		predicates = append(predicates, entsys.RequestIDEQ(input.RequestID))
	}

	if len(predicates) > 0 {
		query.Where(predicates...)
	}

	// Count total
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to count errors", err)
	}

	// Pagination & Order
	logs, err := query.
		Order(ent.Desc(entsys.FieldCreatedAt)).
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		All(ctx)

	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to query errors", err)
	}

	return logs, total, nil
}

// ClearOldErrors deletes errors older than retention days
func (s *SystemErrorService) ClearOldErrors(ctx context.Context, retentionDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	count, err := s.client.SystemError.Delete().
		Where(entsys.CreatedAtLT(cutoff)).
		Exec(ctx)

	if err != nil {
		return 0, errors.NewInternalError("Failed to clear old errors", err)
	}
	return count, nil
}

// Delete deletes a single error entry
func (s *SystemErrorService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.client.SystemError.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Error log not found", err)
		}
		return errors.NewInternalError("Failed to delete error log", err)
	}
	return nil
}
