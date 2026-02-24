package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// Domain represents a single domain instance (e.g. platform:root, tenant:<uuid>).
type Domain struct {
	ent.Schema
}

func (Domain) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (Domain) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(frameEntities.NewUUID).
			Immutable().
			StructTag(`json:"id"`),
		field.String("type_code").
			NotEmpty().
			MaxLen(50).
			Comment("Domain type code reference").
			StructTag(`json:"typeCode"`),
		field.String("key").
			NotEmpty().
			MaxLen(200).
			Comment("Domain key, unique within type").
			StructTag(`json:"key"`),
		field.String("display_name").
			NotEmpty().
			MaxLen(200).
			Comment("Display name").
			StructTag(`json:"displayName"`),
		field.Enum("status").
			Values("active", "inactive", "suspended").
			Default("active").
			Comment("Domain status").
			StructTag(`json:"status"`),
		field.JSON("metadata", map[string]any{}).
			Optional().
			Comment("Domain metadata").
			StructTag(`json:"metadata,omitempty"`),
	}
}

func (Domain) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("domain_type", DomainType.Type).
			Ref("domains").
			Unique().
			Comment("Domain type reference"),
		edge.To("memberships", DomainMembership.Type).
			Comment("Domain memberships"),
		edge.To("bindings", ResourceDomainBinding.Type).
			Comment("Resource bindings to this domain"),
	}
}

func (Domain) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type_code", "key").Unique(),
		index.Fields("status"),
		index.Fields("type_code"),
	}
}
