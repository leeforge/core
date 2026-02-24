package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// AuditLog 审计日志实体
type AuditLog struct {
	ent.Schema
}

// Mixin for AuditLog entity
func (AuditLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("action").
			NotEmpty().
			Comment("操作类型"),
		field.String("resource").
			NotEmpty().
			Comment("资源类型"),
		field.String("resource_id").
			Optional().
			Comment("资源ID"),
		field.JSON("before", map[string]any{}).
			Optional().
			Comment("操作前数据"),
		field.JSON("after", map[string]any{}).
			Optional().
			Comment("操作后数据"),
		field.String("ip_address").
			Optional().
			Comment("IP地址"),
		field.String("user_agent").
			Optional().
			Comment("用户代理"),
	}
}

func (AuditLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Required().
			Ref("audit_logs").
			Comment("操作用户"),
	}
}

func (AuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("action", "resource"),
	}
}
