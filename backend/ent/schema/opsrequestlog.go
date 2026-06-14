package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsRequestLog 运维请求级日志（只追加）。
//
// 由 gateway 插件经 Host.Invoke("ops.report_request") 上报，每次转发一条。
// 与 UsageLog 的区别：UsageLog 面向计费（token/cost），OpsRequestLog 面向运维
// （延迟、状态码、错误体、上游错误），用于实时大盘、错误钻取与告警。
//
// 设计要点：
//   - 高写入日志表，全部用快照字段（int snapshot），不挂外键 edge，避免写入开销
//     与删除约束（与 UsageLog 的 user_id_snapshot 思路一致）。
//   - 错误体/请求体做长度截断由上报方（插件）负责，本表只存。
//   - 必须带保留期清理（见 ops 聚合/清理逻辑），否则增长过快撑爆库。
type OpsRequestLog struct {
	ent.Schema
}

func (OpsRequestLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").Default("").
			Comment("请求追踪 ID，用于错误详情钻取与跨日志关联"),
		field.String("plugin_id").Default("").
			Comment("上报来源插件 ID（如 gateway-openai）"),
		field.String("platform").Default(""),
		field.String("model").Default(""),
		field.String("endpoint").Default("").
			Comment("请求端点，如 /v1/chat/completions"),

		// 归属快照（不挂外键，用户/账号删除后仍可追溯运维归属）
		field.Int("user_id_snapshot").Default(0),
		field.Int("api_key_id_snapshot").Default(0),
		field.Int("account_id_snapshot").Default(0),
		field.Int("group_id_snapshot").Default(0),

		// 结果指标
		field.Bool("success").Default(false),
		field.Int("status_code").Default(0).
			Comment("对外返回的 HTTP 状态码"),
		field.Int("upstream_status_code").Default(0).
			Comment("上游返回的 HTTP 状态码（插件可见，Core 自身拿不到）"),
		field.Int64("duration_ms").Default(0),
		field.Int64("first_token_ms").Default(0).
			Comment("首 token 延迟（流式），非流式为 0"),
		field.Bool("stream").Default(false),

		// token（与计费独立，仅用于运维 token 统计；可能与 UsageLog 重复但口径不同）
		field.Int("input_tokens").Default(0),
		field.Int("output_tokens").Default(0),

		// 错误信息（成功请求留空）
		field.String("error_kind").Default("").
			Comment("错误分类，如 upstream_4xx / upstream_5xx / timeout / rate_limited / internal"),
		field.Text("error_msg").Default("").
			Comment("错误摘要（上报方已截断）"),
		field.Text("error_detail").Default("").
			Comment("错误详情/上游响应体片段（上报方已截断），供钻取"),

		field.Time("created_at").Default(timeNow).Immutable(),
	}
}

func (OpsRequestLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").
			StorageKey("ops_request_log_created_at"),
		index.Fields("success", "created_at").
			StorageKey("ops_request_log_success_created_at"),
		index.Fields("platform", "created_at").
			StorageKey("ops_request_log_platform_created_at"),
		index.Fields("error_kind", "created_at").
			StorageKey("ops_request_log_error_kind_created_at"),
		index.Fields("request_id").
			StorageKey("ops_request_log_request_id"),
	}
}
