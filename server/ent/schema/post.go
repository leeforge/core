package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// Post 文章实体
type Post struct {
	ent.Schema
}

// Mixin for Post entity
func (Post) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{}, // id, created_at, updated_at, deleted_at, etc.
	}
}

func (Post) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			NotEmpty().
			Comment("文章标题"),
		field.Text("content").
			NotEmpty().
			Comment("文章内容"),
		field.String("slug").
			NotEmpty().
			Comment("URL别名"),
		field.Enum("status").
			Values("draft", "published", "archived").
			Default("draft").
			Comment("发布状态"),
		field.Int("view_count").
			Default(0).
			Comment("浏览次数"),
		field.JSON("tags", []string{}).
			Optional().
			Comment("标签"),
	}
}

func (Post) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).
			Required().
			Ref("posts").
			Comment("文章作者"),
		edge.To("media", Media.Type).
			Comment("文章关联的媒体"),
	}
}

func (Post) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "slug").Unique(),
		index.Fields("status", "published_at"),
	}
}
