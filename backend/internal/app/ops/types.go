package ops

import (
	"context"
	"time"
)

// ReportInput 插件经 Host.Invoke("ops.report_request") 上报的请求级指标。
//
// 由 gateway 插件在 Forward 收尾处组装。所有字段都是快照值，Core 直接落库，
// 不做二次校验（capability 已在 host 层校验上报权限）。
type ReportInput struct {
	RequestID          string
	ClientRequestID    string
	PluginID           string
	Platform           string
	Model              string
	UpstreamModel      string
	Endpoint           string
	UserID             int
	APIKeyID           int
	AccountID          int
	GroupID            int
	Success            bool
	StatusCode         int
	UpstreamStatusCode int
	DurationMs         int64
	FirstTokenMs       int64
	Stream             bool
	InputTokens        int
	OutputTokens       int
	ErrorKind          string
	ErrorMsg           string
	ErrorDetail        string
	// CreatedAt 上报方的请求时刻；零值时由 store 用当前时间兜底。
	CreatedAt time.Time
}

// RequestLog 请求级日志领域对象（查询/钻取用）。
type RequestLog struct {
	ID                 int
	RequestID          string
	ClientRequestID    string
	PluginID           string
	Platform           string
	Model              string
	UpstreamModel      string
	Endpoint           string
	UserID             int
	APIKeyID           int
	AccountID          int
	GroupID            int
	Success            bool
	StatusCode         int
	UpstreamStatusCode int
	DurationMs         int64
	FirstTokenMs       int64
	Stream             bool
	InputTokens        int
	OutputTokens       int
	ErrorKind          string
	ErrorMsg           string
	ErrorDetail        string
	CreatedAt          time.Time
}

// RequestLogFilter 错误日志/请求日志查询条件。
type RequestLogFilter struct {
	Page      int
	PageSize  int
	Platform  string
	Model     string
	ErrorKind string
	// OnlyErrors true 时只返回 success=false 的记录（错误日志视图，向后兼容）。
	// 当 SuccessMode 非空时以 SuccessMode 为准，OnlyErrors 被忽略。
	OnlyErrors bool
	// SuccessMode 成功态过滤：""/"all"=全部，"success"=仅成功，"error"=仅失败。
	SuccessMode string
	// MinDurationMs 只看耗时 >= 该值的请求（慢请求筛选，0=不限）。
	MinDurationMs int64
	// SortBy 排序字段："created_at"（默认）或 "duration_ms"。
	SortBy string
	// SortDesc true=降序（默认）。
	SortDesc bool
	Start    *time.Time
	End      *time.Time
}

// RequestLogResult 请求日志查询结果（分页）。
type RequestLogResult struct {
	List     []RequestLog
	Total    int64
	Page     int
	PageSize int
}

// WindowStat 单个聚合窗口的统计（聚合器产出 / 大盘读取）。
type WindowStat struct {
	WindowStart           time.Time
	WindowSeconds         int
	Platform              string
	TotalRequests         int64
	SuccessRequests       int64
	ErrorRequests         int64
	UpstreamErrorRequests int64
	RPS                   float64
	ErrorRate             float64
	P50DurationMs         int64
	P95DurationMs         int64
	P99DurationMs         int64
	TotalInputTokens      int64
	TotalOutputTokens     int64
}

// Overview 实时大盘概览（取最近一个窗口 + 一段趋势）。
type Overview struct {
	// Latest 最近一个已聚合窗口的全平台汇总（可能为零值，表示暂无数据）。
	Latest WindowStat
	// Trend 最近 N 个窗口的全平台汇总，按时间升序，供趋势图。
	Trend []WindowStat
}

// RawAggregate 聚合器从原始日志算出的一个窗口的中间结果。
// 由 store 的聚合查询返回，service 据此写入 WindowStat。
type RawAggregate struct {
	Platform              string
	TotalRequests         int64
	SuccessRequests       int64
	ErrorRequests         int64
	UpstreamErrorRequests int64
	P50DurationMs         int64
	P95DurationMs         int64
	P99DurationMs         int64
	TotalInputTokens      int64
	TotalOutputTokens     int64
}

// ===== 分析（Analytics）：按时间范围从原始日志实时计算 =====

// AnalyticsFilter 分析查询条件（M2 时间范围 + 平台下钻）。
type AnalyticsFilter struct {
	Start    time.Time
	End      time.Time
	Platform string
}

// Percentiles 一组分位数（延迟 / TTFT 通用）。
type Percentiles struct {
	P50     int64   `json:"p50"`
	P90     int64   `json:"p90"`
	P95     int64   `json:"p95"`
	P99     int64   `json:"p99"`
	Max     int64   `json:"max"`
	Avg     float64 `json:"avg"`
	Samples int64   `json:"samples"` // 参与计算的样本数（TTFT 仅流式有值）
}

// HistogramBucket 延迟直方图的一个桶。
type HistogramBucket struct {
	Label string `json:"label"` // 如 "0-50ms"
	Min   int64  `json:"min_ms"`
	Max   int64  `json:"max_ms"` // 0 表示无上限（最后一桶）
	Count int64  `json:"count"`
}

// ErrorClass 错误分布的一类。
type ErrorClass struct {
	Kind  string  `json:"kind"`  // upstream_5xx / client_4xx / gateway_5xx / other
	Label string  `json:"label"` // 展示名
	Count int64   `json:"count"`
	Ratio float64 `json:"ratio"` // 占总错误数比例
}

// ModelTokenStat 单个模型的 token 统计。
type ModelTokenStat struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// PlatformStat 单平台维度汇总（平台下钻）。
type PlatformStat struct {
	Platform      string  `json:"platform"`
	Requests      int64   `json:"requests"`
	ErrorRequests int64   `json:"error_requests"`
	ErrorRate     float64 `json:"error_rate"`
}

// AnalyticsSummary 区间总览。
type AnalyticsSummary struct {
	RangeSeconds    int64   `json:"range_seconds"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	ErrorRate       float64 `json:"error_rate"`
	RPS             float64 `json:"rps"`
	Sampled         bool    `json:"sampled"` // 是否因超采样上限被截断（分位为近似）
}

// Analytics 一次分析查询的完整结果（驱动 M8/M9/M10/M14）。
type Analytics struct {
	Summary           AnalyticsSummary
	Latency           Percentiles
	TTFT              Percentiles
	Histogram         []HistogramBucket
	ErrorDistribution []ErrorClass
	TokensByModel     []ModelTokenStat
	PlatformBreakdown []PlatformStat
}

// Repository ops 域的持久化接口。实现见 infra/store/ops_store.go（仅该层 import ent）。
type Repository interface {
	// Insert 落一条请求级日志。
	Insert(ctx context.Context, in ReportInput) error
	// ListRequestLogs 分页查询请求/错误日志。
	ListRequestLogs(ctx context.Context, f RequestLogFilter) (RequestLogResult, error)
	// GetRequestLog 按 ID 查单条（错误详情钻取）。
	GetRequestLog(ctx context.Context, id int) (RequestLog, error)
	// ListTrace 返回同一 client_request_id 的所有请求日志（按时间升序），即一次请求的重试链路。
	ListTrace(ctx context.Context, clientRequestID string) ([]RequestLog, error)

	// AggregateWindow 从原始日志聚合 [start, end) 区间，按 platform 分组 + 一条全平台汇总（platform=""）。
	AggregateWindow(ctx context.Context, start, end time.Time) ([]RawAggregate, error)
	// QueryAnalytics 按时间范围从原始日志实时计算全套分析指标（M8/M9/M10/M14）。
	QueryAnalytics(ctx context.Context, f AnalyticsFilter) (Analytics, error)
	// ListAccountConcurrency 读取所有账号的并发配置与状态（M7 并发统计）。
	ListAccountConcurrency(ctx context.Context) ([]AccountConcurrencyInfo, error)
	// InsertSystemLogs 批量写入系统日志（M11，由 LogSink worker 调用）。
	InsertSystemLogs(ctx context.Context, entries []SystemLogEntry) error
	// ListSystemLogs 分页查询系统日志。
	ListSystemLogs(ctx context.Context, f SystemLogFilter) (SystemLogResult, error)
	// PurgeSystemLogsBefore 删除 before 之前的系统日志。返回删除条数。
	PurgeSystemLogsBefore(ctx context.Context, before time.Time) (int, error)
	// UpsertWindowStats 写入/更新一个窗口的聚合结果（按 window_start+platform 去重）。
	UpsertWindowStats(ctx context.Context, windowStart time.Time, windowSeconds int, stats []WindowStat) error
	// ListWindowStats 查指定平台（platform=""=全平台汇总）最近 since 之后的窗口，按时间升序。
	ListWindowStats(ctx context.Context, since time.Time, platform string) ([]WindowStat, error)

	// PurgeRequestLogsBefore 删除 before 之前的请求日志（保留期清理）。返回删除条数。
	PurgeRequestLogsBefore(ctx context.Context, before time.Time) (int, error)
	// PurgeWindowStatsBefore 删除 before 之前的聚合窗口。返回删除条数。
	PurgeWindowStatsBefore(ctx context.Context, before time.Time) (int, error)

	// ---- 告警（M3）----
	CreateAlertRule(ctx context.Context, in AlertRuleInput) (AlertRule, error)
	UpdateAlertRule(ctx context.Context, id int, in AlertRuleInput) (AlertRule, error)
	DeleteAlertRule(ctx context.Context, id int) error
	GetAlertRule(ctx context.Context, id int) (AlertRule, error)
	// ListAlertRules enabledOnly=true 只返回启用规则。
	ListAlertRules(ctx context.Context, enabledOnly bool) ([]AlertRule, error)
	SetRuleLastFired(ctx context.Context, id int, t time.Time) error

	CreateAlertEvent(ctx context.Context, ev AlertEvent) (AlertEvent, error)
	ListAlertEvents(ctx context.Context, f AlertEventFilter) (AlertEventResult, error)
	// GetOpenEventForRule 返回该规则当前 firing 中的事件（若有）。
	GetOpenEventForRule(ctx context.Context, ruleID int) (AlertEvent, bool, error)
	ResolveAlertEvent(ctx context.Context, id int, resolvedAt time.Time) error

	CreateSilence(ctx context.Context, in AlertSilenceInput) (AlertSilence, error)
	DeleteSilence(ctx context.Context, id int) error
	ListActiveSilences(ctx context.Context, now time.Time) ([]AlertSilence, error)
	ListSilences(ctx context.Context) ([]AlertSilence, error)
}
