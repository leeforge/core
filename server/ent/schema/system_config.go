package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// SystemConfig 系统配置实体
type SystemConfig struct {
	ent.Schema
}

// Mixin for SystemConfig entity
func (SystemConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{}, // global/platform-scoped metadata
	}
}

func (SystemConfig) Fields() []ent.Field {
	return []ent.Field{
		// BaseEntitySchema already provides: id, created_at, updated_at, deleted_at, etc.
		field.String("key").
			NotEmpty().
			Comment("配置键"),
		field.Text("value").
			NotEmpty().
			Comment("配置值"),
		field.Text("description").
			Optional().
			Comment("配置描述"),
	}
}

func (SystemConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
	}
}
