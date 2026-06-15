// Package moderation 提供内容审核（风控）域用例。
//
// 架构定位（见 docs/architecture/risk-quota-compliance-design.md 第一节）：
//   - 纯 Core：airgate Core 在转发给插件前已读取完整请求体，可在 forwarder 预检阶段
//     抽取待审文本做审核，无需改任何 gateway 插件（与运维 Ops 的"插件上报"根本不同）。
//   - 检测 = 本地敏感词 + OpenAI Moderation API（可单用或串联）。
//
// 本文件定义域类型与接口；落库实现在 infra/store（第二批），检测逻辑纯函数、可独立测试。
package moderation

import (
	"context"
	"time"
)

// Mode 审核模式。
type Mode string

const (
	ModeOff      Mode = "off"       // 不审核
	ModeObserve  Mode = "observe"   // 异步审核，立即放行（命中只记录/通知）
	ModePreBlock Mode = "pre_block" // 同步审核，命中即拒
)

// KeywordMode 关键词与 API 的组合方式。
type KeywordMode string

const (
	KeywordOnly   KeywordMode = "keyword_only"    // 仅本地词库
	KeywordAndAPI KeywordMode = "keyword_and_api" // 先词库后 API（默认）
	APIOnly       KeywordMode = "api_only"        // 仅 OpenAI Moderation API
)

// Source 命中来源。
type Source string

const (
	SourceNone    Source = ""
	SourceKeyword Source = "keyword"
	SourceAPI     Source = "api"
)

// Config 审核配置（来自 settings group="moderation" 的 JSON）。
type Config struct {
	Enabled     bool
	Mode        Mode
	KeywordMode KeywordMode

	// 本地词库：子串、不区分大小写。
	BlockedKeywords []string

	// OpenAI Moderation API。
	APIBaseURL string
	APIModel   string
	APIKeys    []string // 解密后的明文 key（调用方负责解密）
	TimeoutMs  int
	RetryCount int
	// Thresholds 分类 → 阈值（score ≥ 阈值视为命中）。空则用 DefaultThresholds。
	Thresholds map[string]float64

	// 命中拒绝时对外返回。
	BlockStatus  int
	BlockMessage string

	// 自动封禁。
	AutoBanEnabled       bool
	BanThreshold         int
	ViolationWindowHours int
}

// CheckInput 一次审核的输入。
type CheckInput struct {
	RequestID   string
	UserID      int
	Platform    string
	Endpoint    string
	Body        []byte // 完整请求体（Core 已读取）
	ContentType string
}

// Decision 审核判定结果。
type Decision struct {
	Blocked    bool
	Mode       Mode
	Source     Source
	Category   string  // 命中分类（keyword 时为 "keyword"）
	Score      float64 // API 命中分值（keyword 命中为 1）
	Excerpt    string  // 脱敏后的待审文本摘录（落日志用，已截断）
	StatusCode int     // 对外 HTTP 码（命中且 pre_block 时生效）
	Message    string  // 对外消息
}

// LogInput 一条审核日志（落库用，第二批 store 实现）。
type LogInput struct {
	RequestID string
	UserID    int
	Platform  string
	Endpoint  string
	Mode      Mode
	Flagged   bool
	Source    Source
	Category  string
	Score     float64
	Excerpt   string
	CreatedAt time.Time
}

// LogRecord 审核日志读模型（管理后台查询）。
type LogRecord struct {
	ID        int
	RequestID string
	UserID    int
	Platform  string
	Endpoint  string
	Mode      string
	Flagged   bool
	Source    string
	Category  string
	Score     float64
	Excerpt   string
	CreatedAt time.Time
}

// LogFilter 审核日志查询条件。
type LogFilter struct {
	Page     int
	PageSize int
	UserID   int // >0 时按用户过滤
	Platform string
	Flagged  *bool // 非 nil 时按命中过滤
}

// LogResult 审核日志分页结果。
type LogResult struct {
	List     []LogRecord
	Total    int64
	Page     int
	PageSize int
}

// Repository 审核域持久化接口（第二批 ent 实现）。
type Repository interface {
	// InsertLog 落一条审核日志。
	InsertLog(ctx context.Context, in LogInput) error
	// CountViolationsSince 统计某用户 since 之后的命中次数（封禁判定用）。
	CountViolationsSince(ctx context.Context, userID int, since time.Time) (int, error)
	// ListLogs 分页查询审核日志（管理后台）。
	ListLogs(ctx context.Context, f LogFilter) (LogResult, error)
}

// APIModerator 抽象 OpenAI Moderation API 能力，便于测试注入假实现。
type APIModerator interface {
	// Moderate 对 text 调用审核 API；返回最高分分类、分值、是否被 API 标记。
	Moderate(ctx context.Context, text string, cfg Config) (category string, score float64, flagged bool, err error)
}

// ConfigProvider 提供运行期审核配置（settings group="moderation"）。
// 实现负责解密 API key 等敏感项。装配在 bootstrap。
type ConfigProvider interface {
	Load(ctx context.Context) (Config, error)
}

// DefaultThresholds 各分类默认阈值（对齐 sub2api 量级）。
func DefaultThresholds() map[string]float64 {
	return map[string]float64{
		"harassment":             0.90,
		"harassment/threatening": 0.90,
		"hate":                   0.85,
		"hate/threatening":       0.85,
		"self-harm":              0.80,
		"sexual":                 0.65,
		"sexual/minors":          0.50,
		"violence":               0.85,
		"violence/graphic":       0.85,
		"illicit":                0.85,
	}
}
