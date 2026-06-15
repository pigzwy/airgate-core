package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsAlertSilence 告警静音（M3）。在 until 之前抑制指定规则（rule_id=0=全部）的触发。
type OpsAlertSilence struct {
	ent.Schema
}

func (OpsAlertSilence) Fields() []ent.Field {
	return []ent.Field{
		field.Int("rule_id").Default(0).
			Comment("0=静音全部规则"),
		field.String("reason").Default(""),
		field.Time("until"),
		field.Time("created_at").Default(timeNow).Immutable(),
	}
}

func (OpsAlertSilence) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("until"),
	}
}
