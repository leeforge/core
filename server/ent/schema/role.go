package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// Role 角色实体
type Role struct {
	ent.Schema
}

// Mixin for Role entity
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{}, // id, created_at, updated_at, deleted_at, etc.
	}
}

func (Role) Fields() []ent.Field {
	return []ent.Field{
		// 显式定义 UUID 主键（因为有 Edge 关联）
		frameEntities.IDField("id"),
		field.String("name").
			NotEmpty().
			Comment("角色名称").
			StructTag(`json:"name"`),
		field.String("code").
			NotEmpty().
			MaxLen(50).
			Comment("角色编码").
			StructTag(`json:"code"`),
		field.String("description").
			Optional().
			Comment("角色描述").
			StructTag(`json:"description,omitempty"`),
		field.Int("sort").
			Default(0).
			Comment("排序").
			StructTag(`json:"sort"`),
		field.Bool("is_system").
			Default(false).
			Comment("是否系统角色").
			StructTag(`json:"isSystem"`),
		field.JSON("permissions", []string{}).
			Comment("权限列表").
			StructTag(`json:"permissions"`),
		field.String("default_data_scope").
			Default("SELF").
			Comment("默认数据范围").
			StructTag(`json:"defaultDataScope"`),
	}
}

func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).
			Ref("roles").
			Comment("角色下的用户"),
		edge.To("api_keys", ApiKey.Type).
			Comment("关联的API密钥"),
		edge.To("menus", Menu.Type).
			Comment("角色的菜单权限"),

		// 父子角色关系（自引用）
		edge.To("children", Role.Type).
			From("parent").
			Unique().
			Comment("父角色/子角色关系"),
	}
}

func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "name").Unique(),
		index.Fields("owner_domain_id", "code").Unique(),
		// 添加父角色索引，优化查询子角色的性能
		index.Edges("parent"),
	}
}
