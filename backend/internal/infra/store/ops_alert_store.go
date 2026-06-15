package store

import (
	"context"
	"time"

	"github.com/DouDOU-start/airgate-core/ent"
	entalertevent "github.com/DouDOU-start/airgate-core/ent/opsalertevent"
	entalertrule "github.com/DouDOU-start/airgate-core/ent/opsalertrule"
	entalertsilence "github.com/DouDOU-start/airgate-core/ent/opsalertsilence"
	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
)

// ===== 告警规则 =====

func (s *OpsStore) CreateAlertRule(ctx context.Context, in appops.AlertRuleInput) (appops.AlertRule, error) {
	r, err := s.db.OpsAlertRule.Create().
		SetName(in.Name).
		SetMetric(in.Metric).
		SetOperator(in.Operator).
		SetThreshold(in.Threshold).
		SetWindowSeconds(in.WindowSeconds).
		SetSeverity(in.Severity).
		SetEnabled(in.Enabled).
		SetCooldownSeconds(in.CooldownSeconds).
		SetNotifyEmail(in.NotifyEmail).
		SetPlatform(in.Platform).
		Save(ctx)
	if err != nil {
		return appops.AlertRule{}, err
	}
	return toAlertRule(r), nil
}

func (s *OpsStore) UpdateAlertRule(ctx context.Context, id int, in appops.AlertRuleInput) (appops.AlertRule, error) {
	r, err := s.db.OpsAlertRule.UpdateOneID(id).
		SetName(in.Name).
		SetMetric(in.Metric).
		SetOperator(in.Operator).
		SetThreshold(in.Threshold).
		SetWindowSeconds(in.WindowSeconds).
		SetSeverity(in.Severity).
		SetEnabled(in.Enabled).
		SetCooldownSeconds(in.CooldownSeconds).
		SetNotifyEmail(in.NotifyEmail).
		SetPlatform(in.Platform).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return appops.AlertRule{}, appops.ErrAlertRuleNotFound
		}
		return appops.AlertRule{}, err
	}
	return toAlertRule(r), nil
}

func (s *OpsStore) DeleteAlertRule(ctx context.Context, id int) error {
	err := s.db.OpsAlertRule.DeleteOneID(id).Exec(ctx)
	if ent.IsNotFound(err) {
		return appops.ErrAlertRuleNotFound
	}
	return err
}

func (s *OpsStore) GetAlertRule(ctx context.Context, id int) (appops.AlertRule, error) {
	r, err := s.db.OpsAlertRule.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return appops.AlertRule{}, appops.ErrAlertRuleNotFound
		}
		return appops.AlertRule{}, err
	}
	return toAlertRule(r), nil
}

func (s *OpsStore) ListAlertRules(ctx context.Context, enabledOnly bool) ([]appops.AlertRule, error) {
	q := s.db.OpsAlertRule.Query()
	if enabledOnly {
		q = q.Where(entalertrule.EnabledEQ(true))
	}
	rows, err := q.Order(ent.Desc(entalertrule.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.AlertRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAlertRule(r))
	}
	return out, nil
}

func (s *OpsStore) SetRuleLastFired(ctx context.Context, id int, t time.Time) error {
	return s.db.OpsAlertRule.UpdateOneID(id).SetLastFiredAt(t).Exec(ctx)
}

// ===== 告警事件 =====

func (s *OpsStore) CreateAlertEvent(ctx context.Context, ev appops.AlertEvent) (appops.AlertEvent, error) {
	createdAt := ev.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	r, err := s.db.OpsAlertEvent.Create().
		SetRuleID(ev.RuleID).
		SetRuleName(ev.RuleName).
		SetMetric(ev.Metric).
		SetOperator(ev.Operator).
		SetValue(ev.Value).
		SetThreshold(ev.Threshold).
		SetSeverity(ev.Severity).
		SetStatus(ev.Status).
		SetMessage(ev.Message).
		SetCreatedAt(createdAt).
		Save(ctx)
	if err != nil {
		return appops.AlertEvent{}, err
	}
	return toAlertEvent(r), nil
}

func (s *OpsStore) ListAlertEvents(ctx context.Context, f appops.AlertEventFilter) (appops.AlertEventResult, error) {
	q := s.db.OpsAlertEvent.Query()
	if f.Status != "" {
		q = q.Where(entalertevent.StatusEQ(f.Status))
	}
	if f.Severity != "" {
		q = q.Where(entalertevent.SeverityEQ(f.Severity))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return appops.AlertEventResult{}, err
	}
	rows, err := q.
		Order(ent.Desc(entalertevent.FieldCreatedAt)).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		All(ctx)
	if err != nil {
		return appops.AlertEventResult{}, err
	}
	list := make([]appops.AlertEvent, 0, len(rows))
	for _, r := range rows {
		list = append(list, toAlertEvent(r))
	}
	return appops.AlertEventResult{
		List:     list,
		Total:    int64(total),
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

func (s *OpsStore) GetOpenEventForRule(ctx context.Context, ruleID int) (appops.AlertEvent, bool, error) {
	r, err := s.db.OpsAlertEvent.Query().
		Where(
			entalertevent.RuleIDEQ(ruleID),
			entalertevent.StatusEQ("firing"),
		).
		Order(ent.Desc(entalertevent.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return appops.AlertEvent{}, false, nil
		}
		return appops.AlertEvent{}, false, err
	}
	return toAlertEvent(r), true, nil
}

func (s *OpsStore) ResolveAlertEvent(ctx context.Context, id int, resolvedAt time.Time) error {
	return s.db.OpsAlertEvent.UpdateOneID(id).
		SetStatus("resolved").
		SetResolvedAt(resolvedAt).
		Exec(ctx)
}

// ===== 静音 =====

func (s *OpsStore) CreateSilence(ctx context.Context, in appops.AlertSilenceInput) (appops.AlertSilence, error) {
	r, err := s.db.OpsAlertSilence.Create().
		SetRuleID(in.RuleID).
		SetReason(in.Reason).
		SetUntil(in.Until).
		Save(ctx)
	if err != nil {
		return appops.AlertSilence{}, err
	}
	return toAlertSilence(r), nil
}

func (s *OpsStore) DeleteSilence(ctx context.Context, id int) error {
	return s.db.OpsAlertSilence.DeleteOneID(id).Exec(ctx)
}

func (s *OpsStore) ListActiveSilences(ctx context.Context, now time.Time) ([]appops.AlertSilence, error) {
	rows, err := s.db.OpsAlertSilence.Query().
		Where(entalertsilence.UntilGT(now)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.AlertSilence, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAlertSilence(r))
	}
	return out, nil
}

func (s *OpsStore) ListSilences(ctx context.Context) ([]appops.AlertSilence, error) {
	rows, err := s.db.OpsAlertSilence.Query().
		Order(ent.Desc(entalertsilence.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.AlertSilence, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAlertSilence(r))
	}
	return out, nil
}

// ---- 映射 helper ----

func toAlertRule(r *ent.OpsAlertRule) appops.AlertRule {
	return appops.AlertRule{
		ID:              r.ID,
		Name:            r.Name,
		Metric:          r.Metric,
		Operator:        r.Operator,
		Threshold:       r.Threshold,
		WindowSeconds:   r.WindowSeconds,
		Severity:        r.Severity,
		Enabled:         r.Enabled,
		CooldownSeconds: r.CooldownSeconds,
		NotifyEmail:     r.NotifyEmail,
		Platform:        r.Platform,
		LastFiredAt:     r.LastFiredAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func toAlertEvent(r *ent.OpsAlertEvent) appops.AlertEvent {
	return appops.AlertEvent{
		ID:         r.ID,
		RuleID:     r.RuleID,
		RuleName:   r.RuleName,
		Metric:     r.Metric,
		Operator:   r.Operator,
		Value:      r.Value,
		Threshold:  r.Threshold,
		Severity:   r.Severity,
		Status:     r.Status,
		Message:    r.Message,
		CreatedAt:  r.CreatedAt,
		ResolvedAt: r.ResolvedAt,
	}
}

func toAlertSilence(r *ent.OpsAlertSilence) appops.AlertSilence {
	return appops.AlertSilence{
		ID:        r.ID,
		RuleID:    r.RuleID,
		Reason:    r.Reason,
		Until:     r.Until,
		CreatedAt: r.CreatedAt,
	}
}
