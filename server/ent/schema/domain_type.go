package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// DomainType defines a type of domain (e.g. platform, tenant, workspace).
type DomainType struct {
	ent.Schema
}

func (DomainType) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (DomainType) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			NotEmpty().
			Unique().
			MaxLen(50).
			Comment("Domain type code (e.g. platform, tenant, workspace)").
			StructTag(`json:"code"`),
		field.String("plugin_name").
			Optional().
			MaxLen(100).
			Comment("Plugin that registered this domain type").
			StructTag(`json:"pluginName,omitempty"`),
		field.String("display_name").
			NotEmpty().
			MaxLen(200).
			Comment("Display name").
			StructTag(`json:"displayName"`),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("Domain type status").
			StructTag(`json:"status"`),
	}
}

func (DomainType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("domains", Domain.Type).
			Comment("Domains of this type"),
	}
}

func (DomainType) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("status"),
	}
}
