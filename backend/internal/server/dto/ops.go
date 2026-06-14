package dto

// ===== 请求 =====

// OpsOverviewReq 实时大盘查询参数。
type OpsOverviewReq struct {
	// TrendWindows 趋势返回多少个窗口（1min/窗口），默认 60（即最近 1 小时）。
	TrendWindows int `form:"trend_windows"`
}

// OpsErrorLogReq 错误/请求日志查询参数。
type OpsErrorLogReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Platform  string `form:"platform"`
	Model     string `form:"model"`
	ErrorKind string `form:"error_kind"`
	// OnlyErrors 1=只看错误（默认）；0=看全部请求。
	OnlyErrors *bool `form:"only_errors"`
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

// OpsRequestLogResp 单条请求/错误日志。
type OpsRequestLogResp struct {
	ID                 int    `json:"id"`
	RequestID          string `json:"request_id"`
	PluginID           string `json:"plugin_id"`
	Platform           string `json:"platform"`
	Model              string `json:"model"`
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
