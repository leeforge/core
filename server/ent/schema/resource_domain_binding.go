package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// ResourceDomainBinding binds a resource to a domain with an access level.
type ResourceDomainBinding struct {
	ent.Schema
}

func (ResourceDomainBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (ResourceDomainBinding) Fields() []ent.Field {
	return []ent.Field{
		field.String("resource_type").
			NotEmpty().
			MaxLen(100).
			Comment("Resource type (e.g. media, post)").
			StructTag(`json:"resourceType"`),
		field.UUID("resource_id", uuid.UUID{}).
			Comment("Resource ID").
			StructTag(`json:"resourceId"`),
		field.UUID("domain_id", uuid.UUID{}).
			Comment("Target domain ID").
			StructTag(`json:"domainId"`),
		field.Enum("access_level").
			Values("read", "fork", "sync", "manage").
			Default("read").
			Comment("Access level for binding").
			StructTag(`json:"accessLevel"`),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("Binding status").
			StructTag(`json:"status"`),
	}
}

func (ResourceDomainBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("domain", Domain.Type).
			Ref("bindings").
			Field("domain_id").
			Required().
			Unique().
			Comment("Bound domain"),
	}
}

func (ResourceDomainBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("resource_type", "resource_id", "domain_id").Unique(),
		index.Fields("domain_id"),
		index.Fields("resource_type", "resource_id"),
		index.Fields("status"),
	}
}
