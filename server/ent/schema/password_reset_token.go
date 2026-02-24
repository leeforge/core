package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// PasswordResetToken stores one-time JWT reset links.
type PasswordResetToken struct {
	ent.Schema
}

func (PasswordResetToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (PasswordResetToken) Fields() []ent.Field {
	return []ent.Field{
		frameEntities.IDField("id"),
		field.String("jti").Unique().NotEmpty(),
		field.String("token").Unique().NotEmpty(),
		field.String("token_hash").Unique().NotEmpty(),
		field.String("email").NotEmpty(),
		field.Time("expires_at"),
		field.Time("used_at").Optional().Nillable(),
		field.Bool("is_used").Default(false),
		field.String("status").Default("pending"),
	}
}

func (PasswordResetToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Unique(),
	}
}

func (PasswordResetToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("jti").Unique(),
		index.Fields("token").Unique(),
		index.Fields("token_hash").Unique(),
		index.Fields("email"),
		index.Fields("expires_at"),
		index.Fields("status"),
	}
}
