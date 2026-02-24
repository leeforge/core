package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// Media 媒体实体
type Media struct {
	ent.Schema
}

// Mixin 混入 framework 的 Media 定义
func (Media) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
		frameEntities.Media{}, // UUID primary key and media fields
	}
}

// Fields 显式定义 ID 以确保 Ent 识别为 UUID
func (Media) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(frameEntities.NewUUID).
			Immutable(),
		field.UUID("owner_id", uuid.UUID{}).
			Comment("上传者ID").
			StructTag(`json:"ownerId"`),
		field.String("alias").
			Optional().
			Nillable().
			Comment("资源别名"),
		field.UUID("folder_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("媒体目录ID").
			StructTag(`json:"folderId,omitempty"`),
	}
}

// Edges 定义 Media 的边 (本地扩展)
func (Media) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Required().
			Field("owner_id").
			Ref("media").
			Unique().
			Comment("上传者"),
		edge.From("folder", MediaFolder.Type).
			Field("folder_id").
			Ref("files").
			Unique().
			Comment("所属目录"),
		edge.From("posts", Post.Type).
			Ref("media").
			Comment("关联的文章"),
	}
}

func (Media) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "alias").Unique(),
		index.Fields("owner_domain_id", "folder_id", "created_at"),
	}
}

// MediaFolder 媒体目录实体
type MediaFolder struct {
	ent.Schema
}

// Mixin 混入基础租户字段
func (MediaFolder) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

func (MediaFolder) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			Comment("目录名"),
		field.String("path").
			NotEmpty().
			Comment("目录路径"),
		field.Int("depth").
			Default(0).
			Comment("目录深度"),
		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("父目录ID"),
	}
}

func (MediaFolder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", MediaFolder.Type).
			From("parent").
			Unique().
			Field("parent_id").
			Comment("父目录"),
		edge.To("files", Media.Type).
			Comment("目录下文件"),
	}
}

func (MediaFolder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "path").Unique(),
		index.Fields("owner_domain_id", "parent_id", "name").Unique(),
		index.Fields("owner_domain_id", "parent_id"),
	}
}

// MediaFormat 媒体格式实体
type MediaFormat struct {
	ent.Schema
}

func (MediaFormat) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
		frameEntities.MediaFormat{},
	}
}

// Fields 显式定义 ID 以确保 Ent 识别为 UUID
func (MediaFormat) Fields() []ent.Field {
	return []ent.Field{}
}
