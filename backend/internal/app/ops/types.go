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
	PluginID           string
	Platform           string
	Model              string
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
	PluginID           string
	Platform           string
	Model              string
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
	// OnlyErrors true 时只返回 success=false 的记录（错误日志视图）。
	OnlyErrors bool
	Start      *time.Time
	End        *time.Time
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

// Repository ops 域的持久化接口。实现见 infra/store/ops_store.go（仅该层 import ent）。
type Repository interface {
	// Insert 落一条请求级日志。
	Insert(ctx context.Context, in ReportInput) error
	// ListRequestLogs 分页查询请求/错误日志。
	ListRequestLogs(ctx context.Context, f RequestLogFilter) (RequestLogResult, error)
	// GetRequestLog 按 ID 查单条（错误详情钻取）。
	GetRequestLog(ctx context.Context, id int) (RequestLog, error)

	// AggregateWindow 从原始日志聚合 [start, end) 区间，按 platform 分组 + 一条全平台汇总（platform=""）。
	AggregateWindow(ctx context.Context, start, end time.Time) ([]RawAggregate, error)
	// UpsertWindowStats 写入/更新一个窗口的聚合结果（按 window_start+platform 去重）。
	UpsertWindowStats(ctx context.Context, windowStart time.Time, windowSeconds int, stats []WindowStat) error
	// ListWindowStats 查全平台汇总（platform=""）最近 since 之后的窗口，按时间升序。
	ListWindowStats(ctx context.Context, since time.Time) ([]WindowStat, error)

	// PurgeRequestLogsBefore 删除 before 之前的请求日志（保留期清理）。返回删除条数。
	PurgeRequestLogsBefore(ctx context.Context, before time.Time) (int, error)
	// PurgeWindowStatsBefore 删除 before 之前的聚合窗口。返回删除条数。
	PurgeWindowStatsBefore(ctx context.Context, before time.Time) (int, error)
}
