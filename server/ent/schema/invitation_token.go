// PLUGIN: tenant
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// InvitationToken 邀请令牌实体
type InvitationToken struct {
	ent.Schema
}

// Mixin for InvitationToken entity
func (InvitationToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

// Fields of the InvitationToken
func (InvitationToken) Fields() []ent.Field {
	return []ent.Field{
		// 主键 (UUID v7) - 显式定义因为有 Edge 关联
		frameEntities.IDField("id"),

		// 所属租户ID
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("所属租户ID").
			StructTag(`json:"tenantId"`),

		// JWT ID（用于单次激活幂等）
		field.String("jti").
			Unique().
			NotEmpty().
			Comment("邀请令牌 JTI").
			StructTag(`json:"jti"`),

		// Token 字符串（唯一索引）
		field.String("token").
			Unique().
			NotEmpty().
			Comment("邀请令牌字符串").
			StructTag(`json:"token"`),

		// Token 哈希（用于激活时防篡改校验）
		field.String("token_hash").
			Unique().
			NotEmpty().
			Comment("邀请令牌哈希值").
			StructTag(`json:"tokenHash"`),

		// 邀请的邮箱
		field.String("email").
			NotEmpty().
			Comment("被邀请用户的邮箱").
			StructTag(`json:"email"`),

		// 领域类型（tenant/platform/...）
		field.String("domain_type").
			NotEmpty().
			Comment("邀请目标领域类型").
			StructTag(`json:"domainType"`),

		// 领域 key（如 tenant code）
		field.String("domain_key").
			NotEmpty().
			Comment("邀请目标领域标识").
			StructTag(`json:"domainKey"`),

		// 预绑定角色 ID 列表
		field.JSON("role_ids", []string{}).
			Optional().
			Comment("邀请预绑定角色 ID").
			StructTag(`json:"roleIds,omitempty"`),

		// 过期时间（7天后）
		field.Time("expires_at").
			Comment("邀请过期时间").
			StructTag(`json:"expiresAt"`),

		// 使用时间（nil 表示未使用）
		field.Time("used_at").
			Optional().
			Nillable().
			Comment("邀请使用时间").
			StructTag(`json:"usedAt,omitempty"`),

		// 是否已使用
		field.Bool("is_used").
			Default(false).
			Comment("是否已使用").
			StructTag(`json:"isUsed"`),

		// 状态（pending/used/expired）
		field.String("status").
			Default("pending").
			Comment("邀请状态: pending/used/revoked/expired").
			StructTag(`json:"status"`),
	}
}

// Edges of the InvitationToken
func (InvitationToken) Edges() []ent.Edge {
	return []ent.Edge{
		// 关联的激活用户（可选，邀请创建时可能尚未存在用户）
		edge.To("user", User.Type).
			Unique().
			Comment("邀请激活后关联用户"),
	}
}

// Indexes of the InvitationToken
func (InvitationToken) Indexes() []ent.Index {
	return []ent.Index{
		// jti 唯一索引
		index.Fields("jti").Unique(),

		// token 唯一索引（快速查询）
		index.Fields("token").Unique(),

		// token_hash 唯一索引（防重放/防篡改）
		index.Fields("token_hash").Unique(),

		// 租户ID索引
		index.Fields("tenant_id"),

		// 领域类型+领域key组合索引
		index.Fields("domain_type", "domain_key"),

		// 邮箱索引（用于查询某邮箱的邀请）
		index.Fields("email"),

		// 过期时间索引（用于清理过期 token）
		index.Fields("expires_at"),
	}
}
