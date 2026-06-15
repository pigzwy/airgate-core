package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsSystemLog 网关自身运行日志（只追加）。
//
// 由 slog sink（见 app/ops 的 LogSink）异步落库，区别于 OpsRequestLog（请求级业务日志）。
// 用于运维在后台查询网关内部日志（启动、调度、插件、错误堆栈等），多维筛选 + 保留期清理。
//
// 设计要点：
//   - 异步写入、满则丢弃（sink 层背压保护），不阻塞业务日志输出。
//   - 必须带保留期清理，否则增长过快。
type OpsSystemLog struct {
	ent.Schema
}

func (OpsSystemLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("level").Default("").
			Comment("日志级别：debug/info/warn/error"),
		field.String("component").Default("").
			Comment("来源组件/模块（取 slog 的 logger 名或调用方标记）"),
		field.Text("message").Default(""),
		field.String("request_id").Default("").
			Comment("关联请求 ID（若该日志发生在请求上下文）"),
		field.Text("attrs").Default("").
			Comment("结构化字段 JSON 序列化（slog attrs）"),
		field.Time("created_at").Default(timeNow).Immutable(),
	}
}

func (OpsSystemLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").
			StorageKey("ops_system_log_created_at"),
		index.Fields("level", "created_at").
			StorageKey("ops_system_log_level_created_at"),
		index.Fields("component", "created_at").
			StorageKey("ops_system_log_component_created_at"),
		index.Fields("request_id").
			StorageKey("ops_system_log_request_id"),
	}
}
