package datascope

import (
	"context"

	"github.com/google/uuid"
)

// ScopeResolver resolves plugin-defined scope types to filter conditions.
type ScopeResolver interface {
	ScopeTypes() []ScopeType
	Resolve(
		ctx context.Context,
		userID uuid.UUID,
		domainID uuid.UUID,
		scopeType ScopeType,
		scopeValue string,
	) (*FilterCondition, error)
}

func isBaseScopeType(scopeType ScopeType) bool {
	switch scopeType {
	case ScopeAll, ScopeSelf:
		return true
	default:
		return false
	}
}
