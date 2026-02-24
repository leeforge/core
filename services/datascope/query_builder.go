package datascope

import "github.com/google/uuid"

// QueryBuilder 数据范围 SQL 过滤构建器。
type QueryBuilder struct {
	fc *FilterCondition
}

// FilterPredicate is a backend-agnostic predicate description.
type FilterPredicate struct {
	UserIDColumn string
	UserID       uuid.UUID
}

// NewQueryBuilder creates a new QueryBuilder.
func NewQueryBuilder(fc *FilterCondition) *QueryBuilder {
	return &QueryBuilder{fc: fc}
}

// BuildPredicate builds a generic predicate descriptor.
func (qb *QueryBuilder) BuildPredicate(userIDColumn string) *FilterPredicate {
	if qb.fc == nil || qb.fc.IsUnrestricted() {
		return nil
	}

	if qb.fc.Type != ScopeSelf {
		return nil
	}
	return &FilterPredicate{
		UserIDColumn: userIDColumn,
		UserID:       qb.fc.UserID,
	}
}
