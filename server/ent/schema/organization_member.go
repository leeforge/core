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

// OrganizationMember represents a user assignment in an organization.
type OrganizationMember struct {
	ent.Schema
}

func (OrganizationMember) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (OrganizationMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("domain_id", uuid.UUID{}).
			Comment("所属域ID"),
		field.UUID("organization_id", uuid.UUID{}).
			Comment("组织ID"),
		field.UUID("user_id", uuid.UUID{}).
			Comment("用户ID"),
		field.Bool("is_primary").
			Default(false).
			Comment("是否主组织"),
	}
}

func (OrganizationMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", Organization.Type).
			Field("organization_id").
			Unique().
			Required(),
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
			Required(),
	}
}

func (OrganizationMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain_id", "organization_id", "user_id").Unique(),
		index.Fields("domain_id", "organization_id"),
		index.Fields("domain_id", "user_id"),
	}
}
