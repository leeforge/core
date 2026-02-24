package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// APIPermission API权限实体
type APIPermission struct {
	ent.Schema
}

// Mixin for APIPermission entity
func (APIPermission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{}, // global/platform-scoped metadata
	}
}

func (APIPermission) Fields() []ent.Field {
	return []ent.Field{
		field.String("path").
			NotEmpty().
			Comment("API路径"),
		field.Enum("method").
			Values("GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS").
			Default("GET").
			Comment("HTTP方法"),
		field.String("description").
			Optional().
			Comment("API描述"),
		field.Bool("is_public").
			Default(false).
			Comment("是否公开API"),
	}
}

func (APIPermission) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (APIPermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("path", "method").Unique(),
	}
}
