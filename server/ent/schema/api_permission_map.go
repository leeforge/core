package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// APIPermissionMap maps API metadata to permission codes.
type APIPermissionMap struct {
	ent.Schema
}

func (APIPermissionMap) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.GlobalEntitySchema{},
	}
}

func (APIPermissionMap) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("api_id", uuid.UUID{}).
			Comment("API permission ID"),
		field.String("permission_code").
			NotEmpty().
			Comment("Permission code"),
	}
}

func (APIPermissionMap) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("api_id", "permission_code").
			Unique(),
		index.Fields("permission_code"),
	}
}
