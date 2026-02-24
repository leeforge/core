package datascope

import (
	"entgo.io/ent/dialect/sql"
)

// QueryBuilder 数据范围 SQL 过滤构建器。
type QueryBuilder struct {
	fc *FilterCondition
}

// NewQueryBuilder creates a new QueryBuilder.
func NewQueryBuilder(fc *FilterCondition) *QueryBuilder {
	return &QueryBuilder{fc: fc}
}

// BuildPredicate 构建通用 SQL 谓词。
func (qb *QueryBuilder) BuildPredicate(userIDColumn string) func(*sql.Selector) {
	if qb.fc == nil || qb.fc.IsUnrestricted() {
		return nil
	}

	return func(s *sql.Selector) {
		switch qb.fc.Type {
		case ScopeSelf:
			s.Where(sql.EQ(userIDColumn, qb.fc.UserID))
		}
	}
}
