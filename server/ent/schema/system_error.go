package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// SystemError 系统错误日志实体
type SystemError struct {
	ent.Schema
}

func (SystemError) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

func (SystemError) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("error_level").
			Values("info", "warning", "error", "critical").
			Default("error").
			Comment("错误级别").
			StructTag(`json:"errorLevel"`),

		field.String("error_type").
			NotEmpty().
			Comment("错误类型"). // e.g., "panic", "database", "validation"
			StructTag(`json:"errorType"`),

		field.String("error_msg").
			NotEmpty().
			Comment("错误信息").
			StructTag(`json:"errorMsg"`),

		field.Text("error_stack").
			Optional().
			Comment("堆栈信息").
			StructTag(`json:"errorStack,omitempty"`),

		field.String("error_code").
			Optional().
			Comment("错误码").
			StructTag(`json:"errorCode,omitempty"`),

		// 请求上下文
		field.String("request_id").Optional().Comment("请求追踪ID").StructTag(`json:"requestId,omitempty"`),
		field.String("request_method").Optional().StructTag(`json:"requestMethod,omitempty"`),
		field.String("request_path").Optional().StructTag(`json:"requestPath,omitempty"`),
		field.String("request_body").Optional().Comment("请求体").StructTag(`json:"requestBody,omitempty"`),
		field.String("response_body").Optional().Comment("响应体").StructTag(`json:"responseBody,omitempty"`),

		field.UUID("user_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("触发用户ID").
			StructTag(`json:"userId,omitempty"`),
	}
}

func (SystemError) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("system_errors").
			Field("user_id").
			Unique(),
	}
}

func (SystemError) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("error_level"),
		index.Fields("error_type"),
		// index.Fields("created_at"), // 已在 BaseEntitySchema 中定义
		index.Fields("request_id"),
	}
}
