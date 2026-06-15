package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	appmoderation "github.com/DouDOU-start/airgate-core/internal/app/moderation"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// ModerationHandler 内容审核（风控）管理 Handler。
type ModerationHandler struct {
	admin *appmoderation.AdminService
}

// NewModerationHandler 创建 ModerationHandler。
func NewModerationHandler(admin *appmoderation.AdminService) *ModerationHandler {
	return &ModerationHandler{admin: admin}
}

// GetConfig 读取审核配置（api_keys 脱敏）。
func (h *ModerationHandler) GetConfig(c *gin.Context) {
	view, err := h.admin.GetConfig(c.Request.Context())
	if err != nil {
		slog.Error("查询审核配置失败", "error", err)
		response.InternalError(c, "查询失败")
		return
	}
	response.Success(c, toModerationConfigResp(view))
}

// UpdateConfig 写入审核配置（api_keys 明文加密落库，留空保留原值）。
func (h *ModerationHandler) UpdateConfig(c *gin.Context) {
	var req dto.ModerationConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	in := appmoderation.ConfigInput{
		Enabled:              req.Enabled,
		Mode:                 req.Mode,
		KeywordMode:          req.KeywordMode,
		BlockedKeywords:      req.BlockedKeywords,
		APIBaseURL:           req.APIBaseURL,
		APIModel:             req.APIModel,
		APIKeys:              req.APIKeys,
		TimeoutMs:            req.TimeoutMs,
		RetryCount:           req.RetryCount,
		Thresholds:           req.Thresholds,
		BlockStatus:          req.BlockStatus,
		BlockMessage:         req.BlockMessage,
		AutoBanEnabled:       req.AutoBanEnabled,
		BanThreshold:         req.BanThreshold,
		ViolationWindowHours: req.ViolationWindowHours,
	}
	if err := h.admin.SaveConfig(c.Request.Context(), in); err != nil {
		slog.Error("保存审核配置失败", "error", err)
		response.InternalError(c, "保存失败")
		return
	}
	// 返回脱敏后的最新配置
	view, err := h.admin.GetConfig(c.Request.Context())
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, toModerationConfigResp(view))
}

// ListLogs 分页查询审核日志。
func (h *ModerationHandler) ListLogs(c *gin.Context) {
	var q dto.ModerationLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BindError(c, err)
		return
	}
	result, err := h.admin.ListLogs(c.Request.Context(), appmoderation.LogFilter{
		Page:     q.Page,
		PageSize: q.PageSize,
		UserID:   q.UserID,
		Platform: q.Platform,
		Flagged:  q.Flagged,
	})
	if err != nil {
		slog.Error("查询审核日志失败", "error", err)
		response.InternalError(c, "查询失败")
		return
	}
	response.Success(c, response.PagedData(
		toModerationLogResps(result.List),
		result.Total,
		result.Page,
		result.PageSize,
	))
}

// ---- mapper ----

func toModerationConfigResp(v appmoderation.ConfigView) dto.ModerationConfigResp {
	return dto.ModerationConfigResp{
		Enabled:              v.Enabled,
		Mode:                 v.Mode,
		KeywordMode:          v.KeywordMode,
		BlockedKeywords:      v.BlockedKeywords,
		APIBaseURL:           v.APIBaseURL,
		APIModel:             v.APIModel,
		APIKeyHints:          v.APIKeyHints,
		TimeoutMs:            v.TimeoutMs,
		RetryCount:           v.RetryCount,
		Thresholds:           v.Thresholds,
		BlockStatus:          v.BlockStatus,
		BlockMessage:         v.BlockMessage,
		AutoBanEnabled:       v.AutoBanEnabled,
		BanThreshold:         v.BanThreshold,
		ViolationWindowHours: v.ViolationWindowHours,
	}
}

func toModerationLogResp(r appmoderation.LogRecord) dto.ModerationLogResp {
	out := dto.ModerationLogResp{
		ID:        r.ID,
		RequestID: r.RequestID,
		UserID:    r.UserID,
		Platform:  r.Platform,
		Endpoint:  r.Endpoint,
		Mode:      r.Mode,
		Flagged:   r.Flagged,
		Source:    r.Source,
		Category:  r.Category,
		Score:     r.Score,
		Excerpt:   r.Excerpt,
	}
	if !r.CreatedAt.IsZero() {
		out.CreatedAt = r.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toModerationLogResps(items []appmoderation.LogRecord) []dto.ModerationLogResp {
	out := make([]dto.ModerationLogResp, 0, len(items))
	for _, r := range items {
		out = append(out, toModerationLogResp(r))
	}
	return out
}
