package dto

// ===== 请求 =====

// ModerationConfigReq 写入审核配置。api_keys 为明文，留空表示保留原有 key 不变。
type ModerationConfigReq struct {
	Enabled              bool               `json:"enabled"`
	Mode                 string             `json:"mode"`         // off / observe / pre_block
	KeywordMode          string             `json:"keyword_mode"` // keyword_only / keyword_and_api / api_only
	BlockedKeywords      []string           `json:"blocked_keywords"`
	APIBaseURL           string             `json:"api_base_url"`
	APIModel             string             `json:"api_model"`
	APIKeys              []string           `json:"api_keys"`
	TimeoutMs            int                `json:"timeout_ms"`
	RetryCount           int                `json:"retry_count"`
	Thresholds           map[string]float64 `json:"thresholds"`
	BlockStatus          int                `json:"block_status"`
	BlockMessage         string             `json:"block_message"`
	AutoBanEnabled       bool               `json:"auto_ban_enabled"`
	BanThreshold         int                `json:"ban_threshold"`
	ViolationWindowHours int                `json:"violation_window_hours"`
}

// ModerationLogQuery 审核日志查询参数。
type ModerationLogQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	UserID   int    `form:"user_id"`
	Platform string `form:"platform"`
	// Flagged 1=只看命中，0=只看未命中，缺省=全部。
	Flagged *bool `form:"flagged"`
}

// ===== 响应 =====

// ModerationConfigResp 审核配置（api_keys 脱敏为 hint）。
type ModerationConfigResp struct {
	Enabled              bool               `json:"enabled"`
	Mode                 string             `json:"mode"`
	KeywordMode          string             `json:"keyword_mode"`
	BlockedKeywords      []string           `json:"blocked_keywords"`
	APIBaseURL           string             `json:"api_base_url"`
	APIModel             string             `json:"api_model"`
	APIKeyHints          []string           `json:"api_key_hints"`
	TimeoutMs            int                `json:"timeout_ms"`
	RetryCount           int                `json:"retry_count"`
	Thresholds           map[string]float64 `json:"thresholds"`
	BlockStatus          int                `json:"block_status"`
	BlockMessage         string             `json:"block_message"`
	AutoBanEnabled       bool               `json:"auto_ban_enabled"`
	BanThreshold         int                `json:"ban_threshold"`
	ViolationWindowHours int                `json:"violation_window_hours"`
}

// ModerationLogResp 单条审核日志。
type ModerationLogResp struct {
	ID        int     `json:"id"`
	RequestID string  `json:"request_id"`
	UserID    int     `json:"user_id"`
	Platform  string  `json:"platform"`
	Endpoint  string  `json:"endpoint"`
	Mode      string  `json:"mode"`
	Flagged   bool    `json:"flagged"`
	Source    string  `json:"source"`
	Category  string  `json:"category"`
	Score     float64 `json:"score"`
	Excerpt   string  `json:"excerpt"`
	CreatedAt string  `json:"created_at"`
}
