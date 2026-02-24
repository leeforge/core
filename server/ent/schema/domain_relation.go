package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// DomainRelation defines hierarchical relationships between domains.
type DomainRelation struct {
	ent.Schema
}

func (DomainRelation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (DomainRelation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("parent_domain_id", uuid.UUID{}).
			Comment("Parent domain ID").
			StructTag(`json:"parentDomainId"`),
		field.UUID("child_domain_id", uuid.UUID{}).
			Comment("Child domain ID").
			StructTag(`json:"childDomainId"`),
		field.Enum("relation_type").
			Values("contains", "delegates", "inherits").
			Default("contains").
			Comment("Type of hierarchical relation").
			StructTag(`json:"relationType"`),
	}
}

func (DomainRelation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_domain_id", "child_domain_id", "relation_type").Unique(),
		index.Fields("parent_domain_id"),
		index.Fields("child_domain_id"),
	}
}
