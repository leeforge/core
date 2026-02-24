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

// Tenant 租户实体
type Tenant struct {
	ent.Schema
}

// Mixin for Tenant entity
func (Tenant) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			NotEmpty().
			Immutable().
			Comment("租户编码"),
		field.String("name").
			NotEmpty().
			Comment("租户名称"),
		field.String("description").
			Optional().
			Comment("租户描述"),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("租户状态"),
		field.UUID("owner_id", uuid.UUID{}).
			Optional().
			Comment("拥有者用户ID"),
		field.UUID("parent_tenant_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("父租户ID（用于直系层级授权）"),
	}
}

func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Tenant.Type).
			From("parent").
			Unique().
			Field("parent_tenant_id").
			Comment("父子租户层级"),
	}
}

func (Tenant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("status"),
		index.Fields("parent_tenant_id"),
	}
}
