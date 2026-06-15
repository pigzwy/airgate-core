package dto

// ===== 请求 =====

// ComplianceAcceptReq 提交管理员合规确认。
type ComplianceAcceptReq struct {
	// Phrase 须与当前语言要求的确认短语逐字匹配。
	Phrase string `json:"phrase" binding:"required"`
	// Language zh/en，缺省 zh。
	Language string `json:"language"`
}

// ComplianceEnableReq 开启/关闭合规门。
type ComplianceEnableReq struct {
	Enabled bool `json:"enabled"`
}

// ===== 响应 =====

// ComplianceAckResp 一条确认记录。
type ComplianceAckResp struct {
	Version     string `json:"version"`
	AdminUserID int    `json:"admin_user_id"`
	Language    string `json:"language"`
	AcceptedAt  string `json:"accepted_at"`
}

// ComplianceStatusResp 合规确认状态。
type ComplianceStatusResp struct {
	Required     bool   `json:"required"`     // 是否仍需确认
	Enabled      bool   `json:"enabled"`      // 合规门是否开启
	Acknowledged bool   `json:"acknowledged"` // 是否已确认当前版本
	Version      string `json:"version"`
	Language     string `json:"language"`
	Phrase       string `json:"phrase"`      // 当前语言确认短语
	DocumentZh   string `json:"document_zh"` // 中文合规文档全文（markdown）
	DocumentEn   string `json:"document_en"` // 英文合规文档全文（markdown）

	Ack *ComplianceAckResp `json:"ack,omitempty"` // 已确认时附带
}
