package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// ApiKey API密钥实体
type ApiKey struct {
	ent.Schema
}

// Mixin for ApiKey entity
func (ApiKey) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{}, // id, created_at, updated_at, deleted_at, etc.
	}
}

func (ApiKey) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			Comment("密钥名称").
			StructTag(`json:"name"`),
		field.String("description").
			Optional().
			Comment("密钥描述").
			StructTag(`json:"description,omitempty"`),
		field.String("key").
			Unique().
			NotEmpty().
			Comment("API密钥").
			StructTag(`json:"key"`),
		field.String("secret_hash").
			Sensitive().
			NotEmpty().
			Comment("密钥哈希"),
		field.Time("expires_at").
			Optional().
			Comment("过期时间").
			StructTag(`json:"expiresAt,omitempty"`),
		field.JSON("permissions", []string{}).
			Optional().
			Comment("授权权限").
			StructTag(`json:"permissions,omitempty"`),
		field.JSON("data_filters", map[string]any{}).
			Optional().
			Comment("数据过滤条件").
			StructTag(`json:"dataFilters,omitempty"`),
		field.Int("rate_limit").
			Default(60).
			Comment("速率限制(每分钟)").
			StructTag(`json:"rateLimit"`),
		field.Int("daily_limit").
			Default(10000).
			Comment("每日请求限制").
			StructTag(`json:"dailyLimit"`),
		field.UUID("project_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("绑定项目范围（可选）").
			StructTag(`json:"projectId,omitempty"`),
		field.Int64("usage_count").
			Default(0).
			Comment("使用次数").
			StructTag(`json:"usageCount"`),
		field.Time("last_used_at").
			Optional().
			Comment("最后使用时间").
			StructTag(`json:"lastUsedAt,omitempty"`),
		field.String("last_used_ip").
			Optional().
			Comment("最后使用IP").
			StructTag(`json:"lastUsedIp,omitempty"`),
		field.Bool("is_active").
			Default(true).
			Comment("是否激活").
			StructTag(`json:"isActive"`),
	}
}

func (ApiKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Required().
			Ref("api_keys").
			Unique().
			Comment("密钥所有者"),
		edge.To("role", Role.Type).
			Unique().
			Comment("关联角色"),
	}
}

func (ApiKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key"),
		index.Fields("is_active"),
	}
}
