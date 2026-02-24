package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// Dictionary holds the schema definition for the Dictionary entity.
type Dictionary struct {
	ent.Schema
}

// Mixin of the Dictionary.
func (Dictionary) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

// Fields of the Dictionary.
func (Dictionary) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			NotEmpty().
			Comment("字典编码"),
		field.String("name").
			NotEmpty().
			Comment("字典名称"),
		field.String("description").
			Optional().
			Comment("描述"),
		field.Bool("status").
			Default(true).
			Comment("状态"),
	}
}

// Edges of the Dictionary.
func (Dictionary) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", DictionaryDetail.Type).
			Comment("字典项"),
	}
}

// Indexes of the Dictionary.
func (Dictionary) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "code").Unique(),
	}
}
