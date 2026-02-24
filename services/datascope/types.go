package datascope

import "github.com/google/uuid"

// ScopeType 数据范围类型
type ScopeType string

const (
	ScopeAll       ScopeType = "ALL"        // 当前域内全部数据
	ScopeSelf      ScopeType = "SELF"       // 仅自己创建的数据
	ScopeOUSelf    ScopeType = "OU_SELF"    // 当前用户主组织数据
	ScopeOUSubtree ScopeType = "OU_SUBTREE" // 当前用户主组织及其子树数据
)

// ScopePriority 范围优先级（用于合并时取最大）
var ScopePriority = map[ScopeType]int{
	ScopeAll:  2,
	ScopeSelf: 1,
}

// IsKnown reports whether the scope type is supported by current runtime.
func (s ScopeType) IsKnown() bool {
	_, ok := ScopePriority[s]
	return ok
}

// FilterCondition 数据过滤条件
type FilterCondition struct {
	Type   ScopeType `json:"type"`
	UserID uuid.UUID `json:"userId,omitempty"`
}

// IsUnrestricted 是否无限制
func (fc *FilterCondition) IsUnrestricted() bool {
	return fc != nil && fc.Type == ScopeAll
}
