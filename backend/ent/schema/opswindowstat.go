package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsWindowStat 运维聚合窗口统计（预聚合表）。
//
// 由后台聚合器定时（默认 1 分钟）从 OpsRequestLog 滚动计算并写入，供实时大盘
// 快速查询，避免每次查询都扫原始日志。每个 window_start 一行（platform 为空 =
// 全平台汇总；非空 = 按平台细分）。
//
// 参考 sub2api 的 ops_realtime_traffic + ops_window_stats：明细落库 → 后台聚合
// → 大盘读聚合表。
type OpsWindowStat struct {
	ent.Schema
}

func (OpsWindowStat) Fields() []ent.Field {
	return []ent.Field{
		field.Time("window_start").
			Comment("聚合窗口起点（按 1 分钟对齐）"),
		field.Int("window_seconds").Default(60).
			Comment("窗口长度（秒），用于算 RPS"),
		field.String("platform").Default("").
			Comment("空 = 全平台汇总；非空 = 该平台细分"),

		field.Int64("total_requests").Default(0),
		field.Int64("success_requests").Default(0),
		field.Int64("error_requests").Default(0),
		field.Int64("upstream_error_requests").Default(0).
			Comment("上游错误数（status_code >= 500 或 error_kind=upstream_*）"),

		field.Float("rps").Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(12,4)"}).
			Comment("窗口内平均每秒请求数 = total_requests / window_seconds"),
		field.Float("error_rate").Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(8,6)"}).
			Comment("错误率 = error_requests / total_requests"),

		field.Int64("p50_duration_ms").Default(0),
		field.Int64("p95_duration_ms").Default(0),
		field.Int64("p99_duration_ms").Default(0),

		field.Int64("total_input_tokens").Default(0),
		field.Int64("total_output_tokens").Default(0),

		field.Time("created_at").Default(timeNow).Immutable(),
	}
}

func (OpsWindowStat) Indexes() []ent.Index {
	return []ent.Index{
		// 同一窗口同一 platform 唯一，聚合器用 upsert 语义维护
		index.Fields("window_start", "platform").
			Unique().
			StorageKey("ops_window_stat_window_platform"),
	}
}
