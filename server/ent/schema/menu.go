package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// Menu holds the schema definition for the Menu entity.
type Menu struct {
	ent.Schema
}

// Mixin of the Menu.
func (Menu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

// Fields of the Menu.
func (Menu) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Comment("父菜单ID").
			StructTag(`json:"parentId,omitempty"`),
		field.String("path").
			NotEmpty().
			Comment("路由路径").
			StructTag(`json:"path"`),
		field.String("name").
			NotEmpty().
			Comment("路由名称").
			StructTag(`json:"name"`),
		field.String("component").
			Optional().
			Comment("前端组件路径").
			StructTag(`json:"component,omitempty"`),
		field.String("redirect").
			Optional().
			Comment("重定向路径").
			StructTag(`json:"redirect,omitempty"`),
		field.Bool("hidden").
			Default(false).
			Comment("是否隐藏").
			StructTag(`json:"hidden"`),
		field.Bool("affix").
			Default(false).
			Comment("是否固定在标签栏").
			StructTag(`json:"affix"`),
		field.String("icon").
			Optional().
			Comment("菜单图标").
			StructTag(`json:"icon,omitempty"`),
		field.Int("sort").
			Default(0).
			Comment("排序").
			StructTag(`json:"sort"`),
		field.JSON("meta", map[string]any{}).
			Optional().
			Comment("元数据").
			StructTag(`json:"meta,omitempty"`),
		field.JSON("params", []string{}).
			Optional().
			Comment("路由参数").
			StructTag(`json:"params,omitempty"`),
	}
}

// Edges of the Menu.
func (Menu) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("roles", Role.Type).
			Ref("menus").
			Comment("关联的角色"),
		edge.To("children", Menu.Type).
			From("parent").
			Unique().
			Field("parent_id").
			Comment("父级菜单"),
	}
}
