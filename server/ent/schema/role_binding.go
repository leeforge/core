package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// RoleBinding maps subjects to role scopes.
type RoleBinding struct {
	ent.Schema
}

func (RoleBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

func (RoleBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("subject_type").
			Values("user", "service_account").
			Default("user").
			Comment("Role binding subject type"),
		field.UUID("subject_id", uuid.UUID{}).
			Comment("Subject ID"),
		field.String("role_code").
			NotEmpty().
			MaxLen(100).
			Comment("Bound role code"),
		field.UUID("project_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("Optional project boundary"),
		field.Enum("scope_type").
			Values("CURRENT", "DESCENDANTS", "ASSIGNED_SET").
			Default("CURRENT").
			Comment("Scope evaluation type"),
		field.String("scope_value").
			Optional().
			MaxLen(500).
			Comment("Optional scope payload"),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("Binding status"),
	}
}

func (RoleBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "subject_type", "subject_id"),
		index.Fields("owner_domain_id", "role_code", "status"),
	}
}
