package ops

import (
	"context"
	"log/slog"
	"time"
)

// discardWriter 丢弃所有写入，供测试 slog handler 用。
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// countingHandler 是计数用的 slog.Handler，记录 Handle 调用次数。
type countingHandler struct {
	count int
	attrs []slog.Attr
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *countingHandler) Handle(context.Context, slog.Record) error {
	h.count++
	return nil
}
func (h *countingHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return &countingHandler{count: h.count, attrs: append(append([]slog.Attr{}, h.attrs...), as...)}
}
func (h *countingHandler) WithGroup(string) slog.Handler { return h }

// timeAfter 返回 2s 超时通道，供测试等待异步结果。
func timeAfter() <-chan time.Time { return time.After(2 * time.Second) }

// fakeRepo 是 Repository 的测试替身。需要定制返回的测试用例直接给对应函数字段赋值，
// 未赋值的方法返回零值，便于聚焦单个用例。
type fakeRepo struct {
	insertErr error

	listRequestLogsFn func(ctx context.Context, f RequestLogFilter) (RequestLogResult, error)
	getRequestLogFn   func(ctx context.Context, id int) (RequestLog, error)
	listTraceFn       func(ctx context.Context, cid string) ([]RequestLog, error)

	queryAnalyticsFn func(ctx context.Context, f AnalyticsFilter) (Analytics, error)
	listWindowFn     func(ctx context.Context, since time.Time, platform string) ([]WindowStat, error)

	listAccountConcurrencyFn func(ctx context.Context) ([]AccountConcurrencyInfo, error)

	insertSystemLogsFn func(ctx context.Context, entries []SystemLogEntry) error
	listSystemLogsFn   func(ctx context.Context, f SystemLogFilter) (SystemLogResult, error)

	// 告警
	rules          []AlertRule
	createRuleFn   func(ctx context.Context, in AlertRuleInput) (AlertRule, error)
	openEvent      map[int]AlertEvent // ruleID -> firing event
	createdEvents  []AlertEvent
	resolvedEvents []int
	lastFired      map[int]time.Time
	activeSilences []AlertSilence

	purgedLogBefore    *time.Time
	purgedStatBefore   *time.Time
	purgedSysLogBefore *time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		openEvent: map[int]AlertEvent{},
		lastFired: map[int]time.Time{},
	}
}

func (f *fakeRepo) Insert(ctx context.Context, in ReportInput) error { return f.insertErr }

func (f *fakeRepo) ListRequestLogs(ctx context.Context, fl RequestLogFilter) (RequestLogResult, error) {
	if f.listRequestLogsFn != nil {
		return f.listRequestLogsFn(ctx, fl)
	}
	return RequestLogResult{Page: fl.Page, PageSize: fl.PageSize}, nil
}

func (f *fakeRepo) GetRequestLog(ctx context.Context, id int) (RequestLog, error) {
	if f.getRequestLogFn != nil {
		return f.getRequestLogFn(ctx, id)
	}
	return RequestLog{}, nil
}

func (f *fakeRepo) ListTrace(ctx context.Context, cid string) ([]RequestLog, error) {
	if f.listTraceFn != nil {
		return f.listTraceFn(ctx, cid)
	}
	return nil, nil
}

func (f *fakeRepo) AggregateWindow(ctx context.Context, start, end time.Time) ([]RawAggregate, error) {
	return nil, nil
}

func (f *fakeRepo) QueryAnalytics(ctx context.Context, fl AnalyticsFilter) (Analytics, error) {
	if f.queryAnalyticsFn != nil {
		return f.queryAnalyticsFn(ctx, fl)
	}
	return Analytics{}, nil
}

func (f *fakeRepo) ListAccountConcurrency(ctx context.Context) ([]AccountConcurrencyInfo, error) {
	if f.listAccountConcurrencyFn != nil {
		return f.listAccountConcurrencyFn(ctx)
	}
	return nil, nil
}

func (f *fakeRepo) InsertSystemLogs(ctx context.Context, entries []SystemLogEntry) error {
	if f.insertSystemLogsFn != nil {
		return f.insertSystemLogsFn(ctx, entries)
	}
	return nil
}

func (f *fakeRepo) ListSystemLogs(ctx context.Context, fl SystemLogFilter) (SystemLogResult, error) {
	if f.listSystemLogsFn != nil {
		return f.listSystemLogsFn(ctx, fl)
	}
	return SystemLogResult{Page: fl.Page, PageSize: fl.PageSize}, nil
}

func (f *fakeRepo) UpsertWindowStats(ctx context.Context, windowStart time.Time, windowSeconds int, stats []WindowStat) error {
	return nil
}

func (f *fakeRepo) ListWindowStats(ctx context.Context, since time.Time, platform string) ([]WindowStat, error) {
	if f.listWindowFn != nil {
		return f.listWindowFn(ctx, since, platform)
	}
	return nil, nil
}

func (f *fakeRepo) PurgeRequestLogsBefore(ctx context.Context, before time.Time) (int, error) {
	t := before
	f.purgedLogBefore = &t
	return 0, nil
}

func (f *fakeRepo) PurgeWindowStatsBefore(ctx context.Context, before time.Time) (int, error) {
	t := before
	f.purgedStatBefore = &t
	return 0, nil
}

func (f *fakeRepo) PurgeSystemLogsBefore(ctx context.Context, before time.Time) (int, error) {
	t := before
	f.purgedSysLogBefore = &t
	return 0, nil
}

// ---- 告警 ----

func (f *fakeRepo) CreateAlertRule(ctx context.Context, in AlertRuleInput) (AlertRule, error) {
	if f.createRuleFn != nil {
		return f.createRuleFn(ctx, in)
	}
	return AlertRule{Name: in.Name, Metric: in.Metric, Operator: in.Operator, Threshold: in.Threshold}, nil
}

func (f *fakeRepo) UpdateAlertRule(ctx context.Context, id int, in AlertRuleInput) (AlertRule, error) {
	return AlertRule{ID: id, Name: in.Name, Metric: in.Metric, Operator: in.Operator}, nil
}

func (f *fakeRepo) DeleteAlertRule(ctx context.Context, id int) error { return nil }

func (f *fakeRepo) GetAlertRule(ctx context.Context, id int) (AlertRule, error) {
	return AlertRule{ID: id}, nil
}

func (f *fakeRepo) ListAlertRules(ctx context.Context, enabledOnly bool) ([]AlertRule, error) {
	return f.rules, nil
}

func (f *fakeRepo) SetRuleLastFired(ctx context.Context, id int, t time.Time) error {
	f.lastFired[id] = t
	return nil
}

func (f *fakeRepo) CreateAlertEvent(ctx context.Context, ev AlertEvent) (AlertEvent, error) {
	f.createdEvents = append(f.createdEvents, ev)
	return ev, nil
}

func (f *fakeRepo) ListAlertEvents(ctx context.Context, fl AlertEventFilter) (AlertEventResult, error) {
	return AlertEventResult{Page: fl.Page, PageSize: fl.PageSize}, nil
}

func (f *fakeRepo) GetOpenEventForRule(ctx context.Context, ruleID int) (AlertEvent, bool, error) {
	ev, ok := f.openEvent[ruleID]
	return ev, ok, nil
}

func (f *fakeRepo) ResolveAlertEvent(ctx context.Context, id int, resolvedAt time.Time) error {
	f.resolvedEvents = append(f.resolvedEvents, id)
	return nil
}

func (f *fakeRepo) CreateSilence(ctx context.Context, in AlertSilenceInput) (AlertSilence, error) {
	return AlertSilence{RuleID: in.RuleID, Reason: in.Reason, Until: in.Until}, nil
}

func (f *fakeRepo) DeleteSilence(ctx context.Context, id int) error { return nil }

func (f *fakeRepo) ListActiveSilences(ctx context.Context, now time.Time) ([]AlertSilence, error) {
	return f.activeSilences, nil
}

func (f *fakeRepo) ListSilences(ctx context.Context) ([]AlertSilence, error) {
	return f.activeSilences, nil
}

// 编译期断言：fakeRepo 满足 Repository。
var _ Repository = (*fakeRepo)(nil)
