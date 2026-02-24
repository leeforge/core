package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
	frameEntities "github.com/leeforge/framework/entities"
)

// DomainMembership links a subject (user/service_account) to a domain.
type DomainMembership struct {
	ent.Schema
}

func (DomainMembership) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (DomainMembership) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("domain_id", uuid.UUID{}).
			Comment("Domain ID").
			StructTag(`json:"domainId"`),
		field.Enum("subject_type").
			Values("user", "service_account").
			Default("user").
			Comment("Subject type").
			StructTag(`json:"subjectType"`),
		field.UUID("subject_id", uuid.UUID{}).
			Comment("Subject ID (user or service account)").
			StructTag(`json:"subjectId"`),
		field.String("member_role").
			NotEmpty().
			MaxLen(100).
			Default("member").
			Comment("Role within the domain").
			StructTag(`json:"memberRole"`),
		field.Enum("status").
			Values("active", "inactive", "suspended").
			Default("active").
			Comment("Membership status").
			StructTag(`json:"status"`),
		field.Bool("is_default").
			Default(false).
			Comment("Whether this is the user's default domain").
			StructTag(`json:"isDefault"`),
	}
}

func (DomainMembership) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("domain", Domain.Type).
			Ref("memberships").
			Field("domain_id").
			Required().
			Unique().
			Comment("Owning domain"),
	}
}

func (DomainMembership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain_id", "subject_type", "subject_id").Unique(),
		index.Fields("subject_type", "subject_id"),
		index.Fields("subject_id", "is_default"),
		index.Fields("status"),
	}
}
