package dto

// ===== 请求 =====

// OpsOverviewReq 实时大盘查询参数。
type OpsOverviewReq struct {
	// TrendWindows 趋势返回多少个窗口（1min/窗口），默认 60（即最近 1 小时）。
	TrendWindows int `form:"trend_windows"`
	// Platform 平台维度下钻；空=全平台汇总。
	Platform string `form:"platform"`
}

// OpsAnalyticsReq 分析查询参数（时间范围 + 平台下钻）。
type OpsAnalyticsReq struct {
	// RangeSeconds 时间范围（秒），默认 3600（1 小时），上限 7 天。
	RangeSeconds int64 `form:"range_seconds"`
	// Platform 平台维度下钻；空=全平台。
	Platform string `form:"platform"`
}

// OpsErrorLogReq 错误/请求日志查询参数。
type OpsErrorLogReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Platform  string `form:"platform"`
	Model     string `form:"model"`
	ErrorKind string `form:"error_kind"`
	// OnlyErrors 1=只看错误（默认）；0=看全部请求。SuccessMode 非空时以其为准。
	OnlyErrors *bool `form:"only_errors"`
	// SuccessMode 成功态：all/success/error。
	SuccessMode string `form:"success_mode"`
	// MinDurationMs 慢请求筛选：只看耗时 >= 该值。
	MinDurationMs int64 `form:"min_duration_ms"`
	// SortBy 排序字段：created_at（默认）/ duration_ms。
	SortBy string `form:"sort_by"`
	// SortDesc 1=降序（默认）。
	SortDesc *bool `form:"sort_desc"`
	// 时间范围（RFC3339），可选。
	Start string `form:"start"`
	End   string `form:"end"`
}

// ===== 响应 =====

// OpsWindowStatResp 单个聚合窗口。
type OpsWindowStatResp struct {
	WindowStart           string  `json:"window_start"`
	WindowSeconds         int     `json:"window_seconds"`
	Platform              string  `json:"platform"`
	TotalRequests         int64   `json:"total_requests"`
	SuccessRequests       int64   `json:"success_requests"`
	ErrorRequests         int64   `json:"error_requests"`
	UpstreamErrorRequests int64   `json:"upstream_error_requests"`
	RPS                   float64 `json:"rps"`
	ErrorRate             float64 `json:"error_rate"`
	P50DurationMs         int64   `json:"p50_duration_ms"`
	P95DurationMs         int64   `json:"p95_duration_ms"`
	P99DurationMs         int64   `json:"p99_duration_ms"`
	TotalInputTokens      int64   `json:"total_input_tokens"`
	TotalOutputTokens     int64   `json:"total_output_tokens"`
}

// OpsOverviewResp 实时大盘概览。
type OpsOverviewResp struct {
	Latest OpsWindowStatResp   `json:"latest"`
	Trend  []OpsWindowStatResp `json:"trend"`
}

// OpsSystemMetricsResp Core 进程系统资源快照。
type OpsSystemMetricsResp struct {
	Goroutines int           `json:"goroutines"`
	Memory     OpsMemoryResp `json:"memory"`
	DB         OpsDBResp     `json:"db"`
	Redis      OpsRedisResp  `json:"redis"`
	Jobs       []OpsJobResp  `json:"jobs"`
	CapturedAt string        `json:"captured_at"`
}

// OpsDBResp 数据库连接池指标。
type OpsDBResp struct {
	MaxOpenConns      int   `json:"max_open_conns"`
	OpenConns         int   `json:"open_conns"`
	InUse             int   `json:"in_use"`
	Idle              int   `json:"idle"`
	WaitCount         int64 `json:"wait_count"`
	WaitDurationMs    int64 `json:"wait_duration_ms"`
	MaxIdleClosed     int64 `json:"max_idle_closed"`
	MaxLifetimeClosed int64 `json:"max_lifetime_closed"`
}

// OpsMemoryResp 内存指标。
type OpsMemoryResp struct {
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	SysBytes       uint64 `json:"sys_bytes"`
	NumGC          uint32 `json:"num_gc"`
}

// OpsRedisResp Redis 连接池指标。
type OpsRedisResp struct {
	Hits       uint32 `json:"hits"`
	Misses     uint32 `json:"misses"`
	Timeouts   uint32 `json:"timeouts"`
	TotalConns uint32 `json:"total_conns"`
	IdleConns  uint32 `json:"idle_conns"`
	StaleConns uint32 `json:"stale_conns"`
}

// OpsJobResp 单个后台任务状态。
type OpsJobResp struct {
	Name          string `json:"name"`
	Running       bool   `json:"running"`
	LastHeartbeat string `json:"last_heartbeat"`
	LastRunAt     string `json:"last_run_at"`
	LastError     string `json:"last_error"`
}

// ===== 分析（Analytics）响应 =====

// OpsPercentilesResp 分位数组（延迟/TTFT）。
type OpsPercentilesResp struct {
	P50     int64   `json:"p50"`
	P90     int64   `json:"p90"`
	P95     int64   `json:"p95"`
	P99     int64   `json:"p99"`
	Max     int64   `json:"max"`
	Avg     float64 `json:"avg"`
	Samples int64   `json:"samples"`
}

// OpsHistogramBucketResp 延迟直方图桶。
type OpsHistogramBucketResp struct {
	Label string `json:"label"`
	MinMs int64  `json:"min_ms"`
	MaxMs int64  `json:"max_ms"`
	Count int64  `json:"count"`
}

// OpsErrorClassResp 错误分布一类。
type OpsErrorClassResp struct {
	Kind  string  `json:"kind"`
	Label string  `json:"label"`
	Count int64   `json:"count"`
	Ratio float64 `json:"ratio"`
}

// OpsModelTokenResp 单模型 token 统计。
type OpsModelTokenResp struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// OpsPlatformStatResp 平台维度汇总。
type OpsPlatformStatResp struct {
	Platform      string  `json:"platform"`
	Requests      int64   `json:"requests"`
	ErrorRequests int64   `json:"error_requests"`
	ErrorRate     float64 `json:"error_rate"`
}

// OpsAnalyticsSummaryResp 区间总览。
type OpsAnalyticsSummaryResp struct {
	RangeSeconds    int64   `json:"range_seconds"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	ErrorRate       float64 `json:"error_rate"`
	RPS             float64 `json:"rps"`
	Sampled         bool    `json:"sampled"`
}

// OpsAnalyticsResp 分析查询完整响应。
type OpsAnalyticsResp struct {
	Summary           OpsAnalyticsSummaryResp  `json:"summary"`
	Latency           OpsPercentilesResp       `json:"latency"`
	TTFT              OpsPercentilesResp       `json:"ttft"`
	Histogram         []OpsHistogramBucketResp `json:"histogram"`
	ErrorDistribution []OpsErrorClassResp      `json:"error_distribution"`
	TokensByModel     []OpsModelTokenResp      `json:"tokens_by_model"`
	PlatformBreakdown []OpsPlatformStatResp    `json:"platform_breakdown"`
}

// ===== 并发统计（M7）响应 =====

// OpsConcurrencyItemResp 平台/分组维度并发汇总。
type OpsConcurrencyItemResp struct {
	Key     string  `json:"key"`
	Current int     `json:"current"`
	Max     int     `json:"max"`
	Usage   float64 `json:"usage"`
}

// OpsAccountConcurrencyResp 单账号并发明细。
type OpsAccountConcurrencyResp struct {
	AccountID       int     `json:"account_id"`
	Name            string  `json:"name"`
	Platform        string  `json:"platform"`
	Current         int     `json:"current"`
	Max             int     `json:"max"`
	Usage           float64 `json:"usage"`
	State           string  `json:"state"`
	RecoverySeconds int64   `json:"recovery_seconds"`
}

// OpsAccountAvailabilityResp 账号可用性汇总。
type OpsAccountAvailabilityResp struct {
	Active                 int   `json:"active"`
	RateLimited            int   `json:"rate_limited"`
	Degraded               int   `json:"degraded"`
	Disabled               int   `json:"disabled"`
	Total                  int   `json:"total"`
	SoonestRecoverySeconds int64 `json:"soonest_recovery_seconds"`
}

// OpsConcurrencyResp 并发统计完整响应。
type OpsConcurrencyResp struct {
	Availability OpsAccountAvailabilityResp  `json:"availability"`
	ByAccount    []OpsAccountConcurrencyResp `json:"by_account"`
	ByPlatform   []OpsConcurrencyItemResp    `json:"by_platform"`
	ByGroup      []OpsConcurrencyItemResp    `json:"by_group"`
	TotalCurrent int                         `json:"total_current"`
	TotalMax     int                         `json:"total_max"`
}

// ===== 系统日志（M11）=====

// OpsSystemLogReq 系统日志查询参数。
type OpsSystemLogReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Level     string `form:"level"`
	Component string `form:"component"`
	RequestID string `form:"request_id"`
	Keyword   string `form:"keyword"`
	Start     string `form:"start"`
	End       string `form:"end"`
}

// OpsSystemLogResp 单条系统日志。
type OpsSystemLogResp struct {
	ID        int    `json:"id"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Attrs     string `json:"attrs"`
	CreatedAt string `json:"created_at"`
}

// OpsLogLevelResp 当前日志级别。
type OpsLogLevelResp struct {
	Level   string `json:"level"`
	Default string `json:"default"`
	Dropped int64  `json:"dropped"`
}

// OpsLogLevelReq 设置日志级别。
type OpsLogLevelReq struct {
	Level string `json:"level" binding:"required"`
}

// ===== 健康分数（M4）=====

// OpsDiagnosticResp 单条诊断项。
type OpsDiagnosticResp struct {
	Level      string `json:"level"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion"`
}

// OpsHealthResp 健康分数 + 诊断报告。
type OpsHealthResp struct {
	Score       int                 `json:"score"`
	Grade       string              `json:"grade"`
	BusinessSub int                 `json:"business_sub"`
	InfraSub    int                 `json:"infra_sub"`
	Diagnostics []OpsDiagnosticResp `json:"diagnostics"`
	CapturedAt  string              `json:"captured_at"`
}

// ===== 告警（M3）=====

// OpsAlertRuleReq 创建/更新告警规则。
type OpsAlertRuleReq struct {
	Name            string  `json:"name" binding:"required"`
	Metric          string  `json:"metric" binding:"required"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	WindowSeconds   int     `json:"window_seconds"`
	Severity        string  `json:"severity"`
	Enabled         bool    `json:"enabled"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	NotifyEmail     string  `json:"notify_email"`
	Platform        string  `json:"platform"`
}

// OpsAlertRuleResp 告警规则。
type OpsAlertRuleResp struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	WindowSeconds   int     `json:"window_seconds"`
	Severity        string  `json:"severity"`
	Enabled         bool    `json:"enabled"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	NotifyEmail     string  `json:"notify_email"`
	Platform        string  `json:"platform"`
	LastFiredAt     string  `json:"last_fired_at"`
	CreatedAt       string  `json:"created_at"`
}

// OpsAlertEventReq 事件查询参数。
type OpsAlertEventReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Severity string `form:"severity"`
}

// OpsAlertEventResp 告警事件。
type OpsAlertEventResp struct {
	ID         int     `json:"id"`
	RuleID     int     `json:"rule_id"`
	RuleName   string  `json:"rule_name"`
	Metric     string  `json:"metric"`
	Operator   string  `json:"operator"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	Severity   string  `json:"severity"`
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"created_at"`
	ResolvedAt string  `json:"resolved_at"`
}

// OpsAlertSilenceReq 创建静音。
type OpsAlertSilenceReq struct {
	RuleID         int    `json:"rule_id"`
	Reason         string `json:"reason"`
	DurationMinute int    `json:"duration_minute" binding:"required"`
}

// OpsAlertSilenceResp 静音。
type OpsAlertSilenceResp struct {
	ID        int    `json:"id"`
	RuleID    int    `json:"rule_id"`
	Reason    string `json:"reason"`
	Until     string `json:"until"`
	CreatedAt string `json:"created_at"`
}

// OpsSwitchRatePointResp 切换率趋势点（M13）。
type OpsSwitchRatePointResp struct {
	Minute     string  `json:"minute"`
	Sets       int64   `json:"sets"`
	Switches   int64   `json:"switches"`
	SwitchRate float64 `json:"switch_rate"`
}

// OpsRequestLogResp 单条请求/错误日志。
type OpsRequestLogResp struct {
	ID                 int    `json:"id"`
	RequestID          string `json:"request_id"`
	ClientRequestID    string `json:"client_request_id"`
	PluginID           string `json:"plugin_id"`
	Platform           string `json:"platform"`
	Model              string `json:"model"`
	UpstreamModel      string `json:"upstream_model"`
	Endpoint           string `json:"endpoint"`
	UserID             int    `json:"user_id"`
	APIKeyID           int    `json:"api_key_id"`
	AccountID          int    `json:"account_id"`
	GroupID            int    `json:"group_id"`
	Success            bool   `json:"success"`
	StatusCode         int    `json:"status_code"`
	UpstreamStatusCode int    `json:"upstream_status_code"`
	DurationMs         int64  `json:"duration_ms"`
	FirstTokenMs       int64  `json:"first_token_ms"`
	Stream             bool   `json:"stream"`
	InputTokens        int    `json:"input_tokens"`
	OutputTokens       int    `json:"output_tokens"`
	ErrorKind          string `json:"error_kind"`
	ErrorMsg           string `json:"error_msg"`
	ErrorDetail        string `json:"error_detail"`
	CreatedAt          string `json:"created_at"`
}
