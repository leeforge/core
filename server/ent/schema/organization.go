// PLUGIN: ou
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// Organization represents an OU node inside a domain.
type Organization struct {
	ent.Schema
}

func (Organization) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (Organization) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("domain_id", uuid.UUID{}).
			Comment("所属域ID"),
		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("父组织ID"),
		field.String("code").
			NotEmpty().
			Comment("组织编码"),
		field.String("name").
			NotEmpty().
			Comment("组织名称"),
		field.String("path").
			NotEmpty().
			Comment("物化路径"),
	}
}

func (Organization) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Organization.Type).
			From("parent").
			Unique().
			Field("parent_id"),
		edge.To("members", OrganizationMember.Type),
	}
}

func (Organization) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain_id", "code").Unique(),
		index.Fields("domain_id", "path"),
		index.Fields("domain_id", "parent_id"),
	}
}
