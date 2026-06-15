package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsAlertEvent 告警事件（M3）。规则命中产生一条 firing 事件；恢复后置为 resolved。
type OpsAlertEvent struct {
	ent.Schema
}

func (OpsAlertEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int("rule_id").Default(0),
		field.String("rule_name").Default(""),
		field.String("metric").Default(""),
		field.String("operator").Default(""),
		field.Float("value").Default(0),
		field.Float("threshold").Default(0),
		field.String("severity").Default("warning"),
		field.String("status").Default("firing").
			Comment("firing / resolved"),
		field.Text("message").Default(""),
		field.Time("created_at").Default(timeNow).Immutable(),
		field.Time("resolved_at").Optional().Nillable(),
	}
}

func (OpsAlertEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at").
			StorageKey("ops_alert_event_status_created_at"),
		index.Fields("rule_id", "status").
			StorageKey("ops_alert_event_rule_status"),
		index.Fields("created_at").
			StorageKey("ops_alert_event_created_at"),
	}
}
