package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// DictionaryDetail holds the schema definition for the DictionaryDetail entity.
type DictionaryDetail struct {
	ent.Schema
}

// Mixin of the DictionaryDetail.
func (DictionaryDetail) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

// Fields of the DictionaryDetail.
func (DictionaryDetail) Fields() []ent.Field {
	return []ent.Field{
		field.String("label").
			NotEmpty().
			Comment("展示名"),
		field.String("value").
			NotEmpty().
			Comment("字典值"),
		field.String("extend").
			Optional().
			Comment("扩展值(颜色/CSS等)"),
		field.Int("sort").
			Default(0).
			Comment("排序"),
		field.Bool("status").
			Default(true).
			Comment("状态"),
		field.UUID("dictionary_id", uuid.UUID{}).
			Comment("所属字典ID").
			StructTag(`json:"dictionaryId"`),
	}
}

// Edges of the DictionaryDetail.
func (DictionaryDetail) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("dictionary", Dictionary.Type).
			Ref("items").
			Field("dictionary_id").
			Unique().
			Required().
			Comment("所属字典"),
	}
}
