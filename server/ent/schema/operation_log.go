package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	frameEntities "github.com/leeforge/framework/entities"
)

// OperationLog 操作日志实体
type OperationLog struct {
	ent.Schema
}

func (OperationLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		frameEntities.BaseEntitySchema{},
	}
}

// Fields 字段定义
func (OperationLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").
			Optional().
			Comment("请求追踪ID").
			StructTag(`json:"requestId,omitempty"`),

		field.String("method").
			NotEmpty().
			Comment("HTTP方法").
			StructTag(`json:"method"`),

		field.String("path").
			NotEmpty().
			Comment("请求路径").
			StructTag(`json:"path"`),

		field.String("query_params").
			Optional().
			Comment("查询参数").
			StructTag(`json:"queryParams,omitempty"`),

		field.String("body").
			Optional().
			Comment("请求体(已过滤敏感信息)").
			StructTag(`json:"body,omitempty"`),

		field.Int("status").
			Comment("响应状态码").
			StructTag(`json:"status"`),

		field.Int64("latency").
			Comment("响应延迟(纳秒)").
			StructTag(`json:"latency"`),

		field.String("client_ip").
			Comment("客户端IP").
			StructTag(`json:"clientIp"`),

		field.String("user_agent").
			Optional().
			Comment("用户代理").
			StructTag(`json:"userAgent,omitempty"`),

		field.String("error_message").
			Optional().
			Comment("错误信息").
			StructTag(`json:"errorMessage,omitempty"`),

		field.UUID("user_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("操作用户ID").
			StructTag(`json:"userId,omitempty"`),
	}
}

// Edges 关联定义
func (OperationLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("operation_logs").
			Field("user_id").
			Unique(),
	}
}

// Indexes 索引定义
func (OperationLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id"),
		index.Fields("user_id"),
		index.Fields("status"),
		// index.Fields("created_at"), // 已在 BaseEntitySchema 中定义
		index.Fields("path"),
		index.Fields("method"),
	}
}
