// Package compliance 提供管理员合规确认门的领域用例。
//
// 设计要点：
//   - 纯 Core，无独立 ent 表：确认状态复用系统设置键值表（group="compliance"）。
//   - 默认关闭：未显式开启 compliance_gate_enabled 前，门不拦任何请求（避免在前端
//     支持 423 之前锁死所有管理端）。
//   - 按管理员维度版本化确认：合规文档版本号变更后，需重新确认。
package compliance

import (
	"context"

	settings "github.com/DouDOU-start/airgate-core/internal/app/settings"
)

// Version 当前合规文档版本。文档正文变更时必须同步递增此版本号——
// 版本变化会使所有管理员的旧确认失效，需重新阅读并确认。
const Version = "v2026.06.15"

// settings 存储约定。
const (
	settingGroup   = "compliance"              // 设置分组
	keyGateEnabled = "compliance_gate_enabled" // "true" 开启合规门；其余视为关闭
	ackKeyPrefix   = "compliance_ack:"         // 每管理员一条：compliance_ack:{userID}
)

// 确认短语（须逐字匹配，作为"已阅读"的责任隔离凭据）。
const (
	phraseZh = "我已阅读、理解并同意 AirGate 部署与运营合规承诺"
	phraseEn = "I have read, understood, and agree to the AirGate Deployment and Operation Compliance Commitment"
)

// Acknowledgement 一条管理员合规确认记录（落在 settings 的 JSON value）。
type Acknowledgement struct {
	Version     string `json:"version"`
	AdminUserID int    `json:"admin_user_id"`
	IPAddress   string `json:"ip_address"`
	UserAgent   string `json:"user_agent"`
	Language    string `json:"language"`
	AcceptedAt  string `json:"accepted_at"` // RFC3339
}

// Status 合规确认状态（GET 端点 + 中间件共用）。
type Status struct {
	// Required 是否仍需确认：门开启 且 未确认当前版本。门关闭时恒为 false。
	Required bool
	// Enabled 合规门是否开启。
	Enabled bool
	// Acknowledged 是否已确认当前版本。
	Acknowledged bool
	Version      string
	Language     string // 归一化后的语言（zh/en）
	Phrase       string // 当前语言的确认短语
	DocumentZh   string // 中文合规文档全文（markdown）
	DocumentEn   string // 英文合规文档全文（markdown）
	// Ack 已确认时附带的记录（否则 nil）。
	Ack *Acknowledgement
}

// AcceptInput 提交确认入参。
type AcceptInput struct {
	AdminUserID int
	Phrase      string
	Language    string
	IPAddress   string
	UserAgent   string
}

// SettingsPort 合规域所需的设置存取能力，由 app/settings.Service 满足。
// 在本包定义最小接口，便于单元测试注入假实现。
type SettingsPort interface {
	List(ctx context.Context, group string) ([]settings.Setting, error)
	Update(ctx context.Context, items []settings.ItemInput) error
}
