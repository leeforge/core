// PLUGIN: tenant
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// TenantUser represents a user membership in a tenant.
type TenantUser struct {
	ent.Schema
}

func (TenantUser) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (TenantUser) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("所属租户ID").
			StructTag(`json:"tenantId"`),
		field.UUID("user_id", uuid.UUID{}).
			Comment("用户ID"),
		field.String("role").
			Optional().
			Comment("租户内角色"),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("成员状态"),
		field.Bool("is_default").
			Default(false).
			Comment("是否默认租户"),
	}
}

func (TenantUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
			Required().
			Comment("所属用户"),
	}
}

func (TenantUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
