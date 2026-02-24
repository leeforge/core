package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// Permission 权限实体（用于细粒度控制）
type Permission struct {
	ent.Schema
}

func (Permission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (Permission) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			NotEmpty().
			Unique().
			Comment("权限码"),
		field.String("name").
			Optional().
			Comment("权限名称"),
		field.String("description").
			Optional().
			Comment("权限描述"),
		field.Enum("scope").
			Values("api", "ui", "data").
			Default("api").
			Comment("权限范围"),
		field.Enum("status").
			Values("active", "deprecated").
			Default("active").
			Comment("权限状态"),
	}
}

func (Permission) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (Permission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope"),
		index.Fields("status"),
	}
}
