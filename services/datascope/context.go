package datascope

import "context"

type filterConditionKey struct{}

// WithFilterCondition 注入过滤条件到 context
func WithFilterCondition(ctx context.Context, fc *FilterCondition) context.Context {
	return context.WithValue(ctx, filterConditionKey{}, fc)
}

// GetFilterCondition 从 context 获取过滤条件
func GetFilterCondition(ctx context.Context) *FilterCondition {
	if fc, ok := ctx.Value(filterConditionKey{}).(*FilterCondition); ok {
		return fc
	}
	return nil
}

// MustGetFilterCondition 必须获取过滤条件
func MustGetFilterCondition(ctx context.Context) *FilterCondition {
	fc := GetFilterCondition(ctx)
	if fc == nil {
		return &FilterCondition{Type: ScopeSelf}
	}
	return fc
}
