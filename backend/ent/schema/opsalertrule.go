package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsAlertRule 运维告警规则（M3）。
//
// 由后台评估器每分钟读取并对指标求值；命中即产生 OpsAlertEvent 并按需邮件通知。
type OpsAlertRule struct {
	ent.Schema
}

func (OpsAlertRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("metric").NotEmpty().
			Comment("指标键：error_rate/p95_latency_ms/p99_latency_ms/ttft_p95_ms/rps/total_requests/db_pool_usage/redis_timeouts/goroutines/health_score"),
		field.String("operator").Default("gt").
			Comment("比较运算符：gt/gte/lt/lte/eq"),
		field.Float("threshold").Default(0),
		field.Int("window_seconds").Default(300).
			Comment("评估窗口（秒），适用于按时间范围计算的指标"),
		field.String("severity").Default("warning").
			Comment("严重度：info/warning/critical"),
		field.Bool("enabled").Default(true),
		field.Int("cooldown_seconds").Default(600).
			Comment("两次触发之间的最小间隔，防止告警风暴"),
		field.String("notify_email").Default("").
			Comment("通知邮箱，逗号分隔；空则不发邮件"),
		field.String("platform").Default("").
			Comment("平台维度过滤；空=全平台"),
		field.Time("last_fired_at").Optional().Nillable(),
		field.Time("created_at").Default(timeNow).Immutable(),
		field.Time("updated_at").Default(timeNow).UpdateDefault(timeNow),
	}
}

func (OpsAlertRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
	}
}
