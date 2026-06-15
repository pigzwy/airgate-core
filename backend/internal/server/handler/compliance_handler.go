package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	appcompliance "github.com/DouDOU-start/airgate-core/internal/app/compliance"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// ComplianceHandler 管理员合规确认门 Handler。
type ComplianceHandler struct {
	service *appcompliance.Service
}

// NewComplianceHandler 创建 ComplianceHandler。
func NewComplianceHandler(service *appcompliance.Service) *ComplianceHandler {
	return &ComplianceHandler{service: service}
}

// GetStatus 返回当前管理员的合规确认状态（含文档全文与确认短语）。
func (h *ComplianceHandler) GetStatus(c *gin.Context) {
	uid, _ := currentUserID(c) // 管理员 API Key 场景 uid=0，按 0 号记录处理
	st, err := h.service.Status(c.Request.Context(), uid, c.Query("language"))
	if err != nil {
		slog.Error("查询合规确认状态失败", "error", err)
		response.InternalError(c, "查询失败")
		return
	}
	response.Success(c, toComplianceStatusResp(st))
}

// Accept 提交合规确认（逐字短语校验）。
func (h *ComplianceHandler) Accept(c *gin.Context) {
	var req dto.ComplianceAcceptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	uid, _ := currentUserID(c)
	st, err := h.service.Accept(c.Request.Context(), appcompliance.AcceptInput{
		AdminUserID: uid,
		Phrase:      req.Phrase,
		Language:    req.Language,
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, appcompliance.ErrInvalidPhrase) {
			response.BadRequest(c, "确认短语不正确，请逐字输入")
			return
		}
		slog.Error("提交合规确认失败", "error", err)
		response.InternalError(c, "提交失败")
		return
	}
	response.Success(c, toComplianceStatusResp(st))
}

// SetEnabled 开启/关闭合规门（管理员）。返回最新状态。
func (h *ComplianceHandler) SetEnabled(c *gin.Context) {
	var req dto.ComplianceEnableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	if err := h.service.SetEnabled(c.Request.Context(), req.Enabled); err != nil {
		slog.Error("更新合规门开关失败", "error", err)
		response.InternalError(c, "更新失败")
		return
	}
	uid, _ := currentUserID(c)
	st, err := h.service.Status(c.Request.Context(), uid, c.Query("language"))
	if err != nil {
		slog.Error("查询合规确认状态失败", "error", err)
		response.InternalError(c, "查询失败")
		return
	}
	response.Success(c, toComplianceStatusResp(st))
}

// toComplianceStatusResp 领域状态 → 响应 DTO。
func toComplianceStatusResp(st appcompliance.Status) dto.ComplianceStatusResp {
	resp := dto.ComplianceStatusResp{
		Required:     st.Required,
		Enabled:      st.Enabled,
		Acknowledged: st.Acknowledged,
		Version:      st.Version,
		Language:     st.Language,
		Phrase:       st.Phrase,
		DocumentZh:   st.DocumentZh,
		DocumentEn:   st.DocumentEn,
	}
	if st.Ack != nil {
		resp.Ack = &dto.ComplianceAckResp{
			Version:     st.Ack.Version,
			AdminUserID: st.Ack.AdminUserID,
			Language:    st.Ack.Language,
			AcceptedAt:  st.Ack.AcceptedAt,
		}
	}
	return resp
}
