package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModerationLog 内容审核命中日志（只追加）。
//
// 由 Core 在转发前预检阶段（forwarder.checkModeration）命中后落一条，供管理后台
// 审核日志查询、违规计数与自动封禁判定。
//
// 设计要点（与 OpsRequestLog 一致）：
//   - 高写入日志表，归属用快照字段（int snapshot），不挂外键 edge，避免写入开销。
//   - excerpt 为脱敏并截断后的待审文本摘录，不存原文（避免泄露用户内容/机密）。
//   - 需带保留期清理（第二批清理逻辑），避免无界增长。
type ModerationLog struct {
	ent.Schema
}

func (ModerationLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").Default("").
			Comment("请求追踪 ID"),
		field.Int("user_id_snapshot").Default(0).
			Comment("命中用户快照（不挂外键）"),
		field.String("platform").Default(""),
		field.String("endpoint").Default("").
			Comment("请求端点，如 /v1/chat/completions"),
		field.String("mode").Default("").
			Comment("审核模式快照：observe / pre_block"),
		field.Bool("flagged").Default(false).
			Comment("是否命中"),
		field.String("source").Default("").
			Comment("命中来源：keyword / api"),
		field.String("category").Default("").
			Comment("命中分类（keyword 命中为 keyword）"),
		field.Float("score").Default(0).
			Comment("API 命中分值（keyword 命中为 1）"),
		field.Text("excerpt").Default("").
			Comment("脱敏并截断后的待审文本摘录"),
		field.Time("created_at").Default(timeNow).Immutable(),
	}
}

func (ModerationLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").
			StorageKey("moderation_log_created_at"),
		index.Fields("user_id_snapshot", "created_at").
			StorageKey("moderation_log_user_created_at"),
		index.Fields("flagged", "created_at").
			StorageKey("moderation_log_flagged_created_at"),
	}
}
