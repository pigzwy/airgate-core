package ops

import (
	"context"
	"testing"
	"time"
)

func TestCompareMetric(t *testing.T) {
	cases := []struct {
		name      string
		value     float64
		op        string
		threshold float64
		want      bool
	}{
		{"gt 命中", 0.1, "gt", 0.05, true},
		{"gt 未命中", 0.05, "gt", 0.05, false},
		{"gte 边界命中", 0.05, "gte", 0.05, true},
		{"lt 命中", 1, "lt", 2, true},
		{"lte 边界命中", 2, "lte", 2, true},
		{"eq 命中", 3, "eq", 3, true},
		{"eq 未命中", 3, "eq", 4, false},
		{"未知运算符", 1, "neq", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compareMetric(c.value, c.op, c.threshold); got != c.want {
				t.Errorf("compareMetric(%v,%q,%v)=%v, want %v", c.value, c.op, c.threshold, got, c.want)
			}
		})
	}
}

func TestValidateRuleInput(t *testing.T) {
	cases := []struct {
		name    string
		in      AlertRuleInput
		wantErr bool
	}{
		{"正常", AlertRuleInput{Name: "r", Metric: "error_rate", Operator: "gt"}, false},
		{"空名", AlertRuleInput{Name: " ", Metric: "error_rate"}, true},
		{"非法指标", AlertRuleInput{Name: "r", Metric: "nope"}, true},
		{"非法运算符", AlertRuleInput{Name: "r", Metric: "rps", Operator: "xx"}, true},
		{"运算符留空允许", AlertRuleInput{Name: "r", Metric: "rps", Operator: ""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateRuleInput(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("validateRuleInput err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestNormalizeRuleInput(t *testing.T) {
	got := normalizeRuleInput(AlertRuleInput{Name: "r", Metric: "rps"})
	if got.Operator != "gt" || got.Severity != "warning" || got.WindowSeconds != 300 || got.CooldownSeconds != 600 {
		t.Errorf("默认值未补齐: %+v", got)
	}
	// 已给值不应被覆盖
	got2 := normalizeRuleInput(AlertRuleInput{Name: "r", Metric: "rps", Operator: "lt", Severity: "critical", WindowSeconds: 60, CooldownSeconds: 30})
	if got2.Operator != "lt" || got2.Severity != "critical" || got2.WindowSeconds != 60 || got2.CooldownSeconds != 30 {
		t.Errorf("已给值被覆盖: %+v", got2)
	}
}

func newTestService(repo Repository) *Service {
	return NewService(repo, nil)
}

func TestEvaluateAlerts_FireCreatesEvent(t *testing.T) {
	repo := newFakeRepo()
	repo.rules = []AlertRule{{
		ID: 1, Name: "高错误率", Metric: "error_rate", Operator: "gt",
		Threshold: 0.05, WindowSeconds: 300, Severity: "critical", Enabled: true, CooldownSeconds: 600,
	}}
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{Summary: AnalyticsSummary{ErrorRate: 0.2, TotalRequests: 100}}, nil
	}
	svc := newTestService(repo)

	if err := svc.EvaluateAlerts(context.Background()); err != nil {
		t.Fatalf("EvaluateAlerts err: %v", err)
	}
	if len(repo.createdEvents) != 1 {
		t.Fatalf("应产生 1 条 firing 事件，实际 %d", len(repo.createdEvents))
	}
	if repo.createdEvents[0].Status != "firing" || repo.createdEvents[0].RuleID != 1 {
		t.Errorf("事件内容不对: %+v", repo.createdEvents[0])
	}
	if _, ok := repo.lastFired[1]; !ok {
		t.Error("应更新 last_fired")
	}
}

func TestEvaluateAlerts_NotBreachedResolvesOpen(t *testing.T) {
	repo := newFakeRepo()
	repo.rules = []AlertRule{{ID: 7, Name: "r", Metric: "error_rate", Operator: "gt", Threshold: 0.5, Enabled: true}}
	repo.openEvent[7] = AlertEvent{ID: 99, RuleID: 7, Status: "firing"}
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{Summary: AnalyticsSummary{ErrorRate: 0.01, TotalRequests: 100}}, nil
	}
	svc := newTestService(repo)

	if err := svc.EvaluateAlerts(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(repo.createdEvents) != 0 {
		t.Errorf("未越界不应产生事件")
	}
	if len(repo.resolvedEvents) != 1 || repo.resolvedEvents[0] != 99 {
		t.Errorf("应自动恢复事件 99，实际 %v", repo.resolvedEvents)
	}
}

func TestEvaluateAlerts_AlreadyFiringNoDuplicate(t *testing.T) {
	repo := newFakeRepo()
	repo.rules = []AlertRule{{ID: 3, Name: "r", Metric: "error_rate", Operator: "gt", Threshold: 0.05, Enabled: true}}
	repo.openEvent[3] = AlertEvent{ID: 5, RuleID: 3, Status: "firing"}
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{Summary: AnalyticsSummary{ErrorRate: 0.9, TotalRequests: 100}}, nil
	}
	svc := newTestService(repo)
	_ = svc.EvaluateAlerts(context.Background())
	if len(repo.createdEvents) != 0 {
		t.Errorf("已在告警中不应重复产事件")
	}
}

func TestEvaluateAlerts_GlobalSilenceSkips(t *testing.T) {
	repo := newFakeRepo()
	repo.rules = []AlertRule{{ID: 1, Name: "r", Metric: "error_rate", Operator: "gt", Threshold: 0.05, Enabled: true}}
	repo.activeSilences = []AlertSilence{{RuleID: 0, Until: time.Now().Add(time.Hour)}}
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{Summary: AnalyticsSummary{ErrorRate: 0.9, TotalRequests: 100}}, nil
	}
	svc := newTestService(repo)
	_ = svc.EvaluateAlerts(context.Background())
	if len(repo.createdEvents) != 0 {
		t.Errorf("全局静音应跳过评估")
	}
}

func TestEvaluateAlerts_CooldownSkips(t *testing.T) {
	repo := newFakeRepo()
	rule := AlertRule{ID: 2, Name: "r", Metric: "error_rate", Operator: "gt", Threshold: 0.05, Enabled: true, CooldownSeconds: 600}
	last := time.Now().Add(-1 * time.Minute) // 冷却期内
	rule.LastFiredAt = &last
	repo.rules = []AlertRule{rule}
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{Summary: AnalyticsSummary{ErrorRate: 0.9, TotalRequests: 100}}, nil
	}
	svc := newTestService(repo)
	_ = svc.EvaluateAlerts(context.Background())
	if len(repo.createdEvents) != 0 {
		t.Errorf("冷却期内不应再次触发")
	}
}
