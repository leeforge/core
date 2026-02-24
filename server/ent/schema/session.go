package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// Session 会话实体（用于JWT管理）
type Session struct {
	ent.Schema
}

// Mixin for Session entity
func (Session) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").
			Sensitive().
			NotEmpty().
			Comment("令牌哈希"),
		field.Time("expires_at").
			Comment("过期时间"),
		field.String("ip_address").
			Optional().
			Comment("登录IP"),
		field.String("user_agent").
			Optional().
			Comment("用户代理"),
		field.Bool("is_revoked").
			Default(false).
			Comment("是否撤销"),
	}
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Required().
			Ref("sessions").
			Comment("会话用户"),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash"),
		index.Fields("expires_at"),
		index.Fields("is_revoked"),
	}
}
