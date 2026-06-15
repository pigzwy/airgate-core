package ops

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ===== 告警闭环（M3）：规则 → 评估 → 事件 → 邮件 → 静音 =====

// AlertRule 告警规则领域对象。
type AlertRule struct {
	ID              int
	Name            string
	Metric          string
	Operator        string
	Threshold       float64
	WindowSeconds   int
	Severity        string
	Enabled         bool
	CooldownSeconds int
	NotifyEmail     string
	Platform        string
	LastFiredAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AlertRuleInput 创建/更新规则入参。
type AlertRuleInput struct {
	Name            string
	Metric          string
	Operator        string
	Threshold       float64
	WindowSeconds   int
	Severity        string
	Enabled         bool
	CooldownSeconds int
	NotifyEmail     string
	Platform        string
}

// AlertEvent 告警事件领域对象。
type AlertEvent struct {
	ID         int
	RuleID     int
	RuleName   string
	Metric     string
	Operator   string
	Value      float64
	Threshold  float64
	Severity   string
	Status     string // firing / resolved
	Message    string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// AlertEventFilter 事件查询条件。
type AlertEventFilter struct {
	Page     int
	PageSize int
	Status   string // firing / resolved / 空=全部
	Severity string
}

// AlertEventResult 事件分页结果。
type AlertEventResult struct {
	List     []AlertEvent
	Total    int64
	Page     int
	PageSize int
}

// AlertSilence 静音规则。
type AlertSilence struct {
	ID        int
	RuleID    int // 0=全部规则
	Reason    string
	Until     time.Time
	CreatedAt time.Time
}

// AlertSilenceInput 创建静音入参。
type AlertSilenceInput struct {
	RuleID int
	Reason string
	Until  time.Time
}

// Notifier 告警通知发送器（邮件）。由 bootstrap 用 settings SMTP 装配。
type Notifier interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SetNotifier 注入告警通知器。装配期调用一次。
func (s *Service) SetNotifier(n Notifier) {
	s.notifier = n
}

// ---- 规则 CRUD ----

func (s *Service) CreateAlertRule(ctx context.Context, in AlertRuleInput) (AlertRule, error) {
	if err := validateRuleInput(in); err != nil {
		return AlertRule{}, err
	}
	return s.repo.CreateAlertRule(ctx, normalizeRuleInput(in))
}

func (s *Service) UpdateAlertRule(ctx context.Context, id int, in AlertRuleInput) (AlertRule, error) {
	if err := validateRuleInput(in); err != nil {
		return AlertRule{}, err
	}
	return s.repo.UpdateAlertRule(ctx, id, normalizeRuleInput(in))
}

func (s *Service) DeleteAlertRule(ctx context.Context, id int) error {
	return s.repo.DeleteAlertRule(ctx, id)
}

func (s *Service) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	return s.repo.ListAlertRules(ctx, false)
}

// ---- 事件 ----

func (s *Service) ListAlertEvents(ctx context.Context, f AlertEventFilter) (AlertEventResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	return s.repo.ListAlertEvents(ctx, f)
}

// ResolveAlertEvent 手动解决一条事件。
func (s *Service) ResolveAlertEvent(ctx context.Context, id int) error {
	return s.repo.ResolveAlertEvent(ctx, id, time.Now())
}

// ---- 静音 ----

func (s *Service) CreateSilence(ctx context.Context, in AlertSilenceInput) (AlertSilence, error) {
	if in.Until.IsZero() || in.Until.Before(time.Now()) {
		return AlertSilence{}, ErrInvalidTimeRange
	}
	return s.repo.CreateSilence(ctx, in)
}

func (s *Service) DeleteSilence(ctx context.Context, id int) error {
	return s.repo.DeleteSilence(ctx, id)
}

func (s *Service) ListSilences(ctx context.Context) ([]AlertSilence, error) {
	return s.repo.ListSilences(ctx)
}

// ---- 评估器 ----

// alertLockKey 评估分布式锁（多实例下只让一个实例评估）。
const alertLockKey = "ops:alert:eval:lock"
const alertLockTTL = 50 * time.Second

// EvaluateAlerts 评估所有启用规则，产出/恢复事件并通知。后台循环每分钟调一次。
func (s *Service) EvaluateAlerts(ctx context.Context) error {
	// 分布式锁（best-effort）。rdb 不可用或抢锁失败则跳过本轮。
	if s.rdb != nil {
		ok, err := s.rdb.SetNX(ctx, alertLockKey, "1", alertLockTTL).Result()
		if err == nil && !ok {
			return nil // 其他实例正在评估
		}
	}

	rules, err := s.repo.ListAlertRules(ctx, true)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	silences, err := s.repo.ListActiveSilences(ctx, time.Now())
	if err != nil {
		return err
	}
	globalSilence := false
	silenced := map[int]bool{}
	for _, sl := range silences {
		if sl.RuleID == 0 {
			globalSilence = true
		} else {
			silenced[sl.RuleID] = true
		}
	}

	for _, rule := range rules {
		if globalSilence || silenced[rule.ID] {
			continue
		}
		value, ok := s.resolveMetric(ctx, rule)
		if !ok {
			continue
		}
		breached := compareMetric(value, rule.Operator, rule.Threshold)
		open, hasOpen, err := s.repo.GetOpenEventForRule(ctx, rule.ID)
		if err != nil {
			s.logger.Warn("ops_alert_get_open_event_failed", "rule_id", rule.ID, "error", err)
			continue
		}

		if breached {
			if hasOpen {
				continue // 已在告警中，不重复产事件
			}
			// 冷却期内不再触发
			if rule.LastFiredAt != nil && time.Since(*rule.LastFiredAt) < time.Duration(rule.CooldownSeconds)*time.Second {
				continue
			}
			msg := formatAlertMessage(rule, value)
			ev := AlertEvent{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Metric:    rule.Metric,
				Operator:  rule.Operator,
				Value:     value,
				Threshold: rule.Threshold,
				Severity:  rule.Severity,
				Status:    "firing",
				Message:   msg,
				CreatedAt: time.Now(),
			}
			if _, err := s.repo.CreateAlertEvent(ctx, ev); err != nil {
				s.logger.Warn("ops_alert_create_event_failed", "rule_id", rule.ID, "error", err)
				continue
			}
			_ = s.repo.SetRuleLastFired(ctx, rule.ID, time.Now())
			s.notifyAlert(ctx, rule, msg, false)
		} else if hasOpen {
			// 恢复
			if err := s.repo.ResolveAlertEvent(ctx, open.ID, time.Now()); err != nil {
				s.logger.Warn("ops_alert_resolve_failed", "event_id", open.ID, "error", err)
				continue
			}
			s.notifyAlert(ctx, rule, fmt.Sprintf("规则「%s」已恢复（当前值 %.2f）", rule.Name, value), true)
		}
	}
	return nil
}

// resolveMetric 计算规则指标的当前值。
func (s *Service) resolveMetric(ctx context.Context, rule AlertRule) (float64, bool) {
	switch rule.Metric {
	case "error_rate", "p95_latency_ms", "p99_latency_ms", "ttft_p95_ms", "rps", "total_requests":
		win := int64(rule.WindowSeconds)
		if win <= 0 {
			win = 300
		}
		a, err := s.repo.QueryAnalytics(ctx, AnalyticsFilter{
			Start:    time.Now().Add(-time.Duration(win) * time.Second),
			End:      time.Now(),
			Platform: rule.Platform,
		})
		if err != nil {
			return 0, false
		}
		switch rule.Metric {
		case "error_rate":
			return a.Summary.ErrorRate, true
		case "p95_latency_ms":
			return float64(a.Latency.P95), true
		case "p99_latency_ms":
			return float64(a.Latency.P99), true
		case "ttft_p95_ms":
			return float64(a.TTFT.P95), true
		case "rps":
			return a.Summary.RPS, true
		case "total_requests":
			return float64(a.Summary.TotalRequests), true
		}
	case "db_pool_usage":
		m := s.SystemMetrics(ctx)
		if m.DB.MaxOpenConns <= 0 {
			return 0, true
		}
		return float64(m.DB.InUse) / float64(m.DB.MaxOpenConns), true
	case "redis_timeouts":
		m := s.SystemMetrics(ctx)
		return float64(m.Redis.Timeouts), true
	case "goroutines":
		m := s.SystemMetrics(ctx)
		return float64(m.Goroutines), true
	case "health_score":
		h, err := s.Health(ctx)
		if err != nil {
			return 0, false
		}
		return float64(h.Score), true
	}
	return 0, false
}

// compareMetric 按运算符比较。
func compareMetric(value float64, op string, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	}
	return false
}

func formatAlertMessage(rule AlertRule, value float64) string {
	return fmt.Sprintf("规则「%s」触发：%s %s %.2f（当前值 %.2f，严重度 %s）",
		rule.Name, rule.Metric, rule.Operator, rule.Threshold, value, rule.Severity)
}

// notifyAlert 发送邮件通知（best-effort）。
func (s *Service) notifyAlert(ctx context.Context, rule AlertRule, message string, resolved bool) {
	if s.notifier == nil || strings.TrimSpace(rule.NotifyEmail) == "" {
		return
	}
	subjectPrefix := "[告警]"
	if resolved {
		subjectPrefix = "[恢复]"
	}
	subject := fmt.Sprintf("%s %s", subjectPrefix, rule.Name)
	body := fmt.Sprintf("<p>%s</p><p>时间：%s</p>", message, time.Now().Format(time.RFC3339))
	for _, addr := range strings.Split(rule.NotifyEmail, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if err := s.notifier.Send(ctx, addr, subject, body); err != nil {
			s.logger.Warn("ops_alert_notify_failed", "rule_id", rule.ID, "to", addr, "error", err)
		}
	}
}

// runAlertLoop 后台告警评估循环。
func (s *Service) runAlertLoop(ctx context.Context) {
	var job *JobHandle
	if s.jobs != nil {
		job = s.jobs.Register("ops_alert")
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if job != nil {
				job.Heartbeat()
			}
			if err := s.EvaluateAlerts(ctx); err != nil {
				s.logger.Warn("ops_alert_evaluate_failed", "error", err)
				if job != nil {
					job.MarkError(err)
				}
			} else if job != nil {
				job.MarkRun()
			}
			// 邮件日报（M15）：到点且当天未发过则发送。
			s.maybeSendDailyReport(ctx)
		}
	}
}

// maybeSendDailyReport 到达配置的整点且当天未发送时，发送一封 24h 运维日报。
func (s *Service) maybeSendDailyReport(ctx context.Context) {
	if s.notifier == nil {
		return
	}
	cfg := s.dailyReportConfig(ctx)
	if !cfg.enabled || strings.TrimSpace(cfg.email) == "" {
		return
	}
	now := time.Now()
	if now.Hour() != cfg.hour {
		return
	}
	today := now.Format("2006-01-02")
	if s.lastReportDate == today {
		return
	}
	// 分布式去重：多实例下用 Redis 标记当天已发。
	if s.rdb != nil {
		ok, err := s.rdb.SetNX(ctx, "ops:report:sent:"+today, "1", 26*time.Hour).Result()
		if err == nil && !ok {
			s.lastReportDate = today
			return
		}
	}

	a, err := s.repo.QueryAnalytics(ctx, AnalyticsFilter{
		Start: now.Add(-24 * time.Hour),
		End:   now,
	})
	if err != nil {
		s.logger.Warn("ops_daily_report_query_failed", "error", err)
		return
	}
	subject := fmt.Sprintf("[运维日报] %s", today)
	body := fmt.Sprintf(
		"<h3>近 24 小时运维概览</h3>"+
			"<ul>"+
			"<li>总请求：%d</li>"+
			"<li>成功：%d，失败：%d，错误率：%.2f%%</li>"+
			"<li>P95 延迟：%d ms，P99：%d ms，最大：%d ms</li>"+
			"<li>TTFT P95：%d ms</li>"+
			"</ul><p>生成时间：%s</p>",
		a.Summary.TotalRequests, a.Summary.SuccessRequests, a.Summary.ErrorRequests,
		a.Summary.ErrorRate*100, a.Latency.P95, a.Latency.P99, a.Latency.Max, a.TTFT.P95,
		now.Format(time.RFC3339))

	for _, addr := range strings.Split(cfg.email, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if err := s.notifier.Send(ctx, addr, subject, body); err != nil {
			s.logger.Warn("ops_daily_report_send_failed", "to", addr, "error", err)
		}
	}
	s.lastReportDate = today
}

// ---- 校验/规范化 ----

var validMetrics = map[string]bool{
	"error_rate": true, "p95_latency_ms": true, "p99_latency_ms": true,
	"ttft_p95_ms": true, "rps": true, "total_requests": true,
	"db_pool_usage": true, "redis_timeouts": true, "goroutines": true,
	"health_score": true,
}

var validOperators = map[string]bool{"gt": true, "gte": true, "lt": true, "lte": true, "eq": true}

func validateRuleInput(in AlertRuleInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return ErrInvalidAlertRule
	}
	if !validMetrics[in.Metric] {
		return ErrInvalidAlertRule
	}
	if in.Operator != "" && !validOperators[in.Operator] {
		return ErrInvalidAlertRule
	}
	return nil
}

func normalizeRuleInput(in AlertRuleInput) AlertRuleInput {
	if in.Operator == "" {
		in.Operator = "gt"
	}
	if in.Severity == "" {
		in.Severity = "warning"
	}
	if in.WindowSeconds <= 0 {
		in.WindowSeconds = 300
	}
	if in.CooldownSeconds <= 0 {
		in.CooldownSeconds = 600
	}
	return in
}
