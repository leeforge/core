package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	frameEntities "github.com/leeforge/framework/entities"
)

// User 用户实体
type User struct {
	ent.Schema
}

// Mixin for User entity
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{}, // id, created_at, updated_at, deleted_at, etc.
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		// BaseEntitySchema already provides: id, created_at, updated_at, deleted_at, etc.
		field.String("username").
			NotEmpty().
			Comment("用户名").
			StructTag(`json:"username"`),
		field.String("email").
			NotEmpty().
			Comment("邮箱").
			StructTag(`json:"email"`),
		field.String("password_hash").
			Sensitive().
			Optional().
			Comment("密码哈希（邀请创建时可为空）"),
		field.String("nickname").
			Default("系统用户").
			Comment("昵称").
			StructTag(`json:"nickname"`),
		field.String("phone").
			Optional().
			Comment("手机号").
			StructTag(`json:"phone,omitempty"`),
		field.String("avatar").
			Optional().
			Comment("头像URL").
			StructTag(`json:"avatar,omitempty"`),
		field.Bool("is_super_admin").
			Default(false).
			Comment("是否超级管理员").
			StructTag(`json:"isSuperAdmin"`),
		field.Enum("status").
			Values("active", "inactive", "suspended").
			Default("active").
			Comment("用户状态").
			StructTag(`json:"status"`),
		field.JSON("metadata", map[string]string{}).
			Optional().
			Comment("元数据").
			StructTag(`json:"metadata,omitempty"`),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type).
			Comment("用户角色"),
		edge.To("posts", Post.Type).
			Comment("用户发布的文章"),
		edge.To("media", Media.Type).
			Comment("用户上传的媒体"),
		edge.To("api_keys", ApiKey.Type).
			Comment("用户的API密钥"),
		edge.To("sessions", Session.Type).
			Comment("用户的会话"),
		edge.To("audit_logs", AuditLog.Type).
			Comment("用户的审计日志"),
		edge.To("operation_logs", OperationLog.Type).
			Comment("用户的操作日志"),
		edge.To("system_errors", SystemError.Type).
			Comment("用户的系统错误日志"),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_domain_id", "username").Unique(),
		index.Fields("owner_domain_id", "email").Unique(),
	}
}
