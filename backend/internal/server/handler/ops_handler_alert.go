package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// ListAlertRules 返回所有告警规则。
func (h *OpsHandler) ListAlertRules(c *gin.Context) {
	rules, err := h.service.ListAlertRules(c.Request.Context())
	if err != nil {
		code, msg := h.handleError("查询告警规则失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsAlertRuleResps(rules))
}

// CreateAlertRule 创建告警规则。
func (h *OpsHandler) CreateAlertRule(c *gin.Context) {
	var req dto.OpsAlertRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	rule, err := h.service.CreateAlertRule(c.Request.Context(), toAlertRuleInput(req))
	if err != nil {
		code, msg := h.handleError("创建告警规则失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsAlertRuleResp(rule))
}

// UpdateAlertRule 更新告警规则。
func (h *OpsHandler) UpdateAlertRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的规则 ID")
		return
	}
	var req dto.OpsAlertRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	rule, err := h.service.UpdateAlertRule(c.Request.Context(), id, toAlertRuleInput(req))
	if err != nil {
		code, msg := h.handleError("更新告警规则失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsAlertRuleResp(rule))
}

// DeleteAlertRule 删除告警规则。
func (h *OpsHandler) DeleteAlertRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的规则 ID")
		return
	}
	if err := h.service.DeleteAlertRule(c.Request.Context(), id); err != nil {
		code, msg := h.handleError("删除告警规则失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ListAlertEvents 分页返回告警事件。
func (h *OpsHandler) ListAlertEvents(c *gin.Context) {
	var req dto.OpsAlertEventReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}
	result, err := h.service.ListAlertEvents(c.Request.Context(), appops.AlertEventFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   req.Status,
		Severity: req.Severity,
	})
	if err != nil {
		code, msg := h.handleError("查询告警事件失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, response.PagedData(
		toOpsAlertEventResps(result.List),
		result.Total,
		result.Page,
		result.PageSize,
	))
}

// ResolveAlertEvent 手动解决一条告警事件。
func (h *OpsHandler) ResolveAlertEvent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的事件 ID")
		return
	}
	if err := h.service.ResolveAlertEvent(c.Request.Context(), id); err != nil {
		code, msg := h.handleError("解决告警事件失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, gin.H{"resolved": true})
}

// ListSilences 返回所有静音。
func (h *OpsHandler) ListSilences(c *gin.Context) {
	silences, err := h.service.ListSilences(c.Request.Context())
	if err != nil {
		code, msg := h.handleError("查询静音失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsAlertSilenceResps(silences))
}

// CreateSilence 创建静音。
func (h *OpsHandler) CreateSilence(c *gin.Context) {
	var req dto.OpsAlertSilenceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	sil, err := h.service.CreateSilence(c.Request.Context(), appops.AlertSilenceInput{
		RuleID: req.RuleID,
		Reason: req.Reason,
		Until:  time.Now().Add(time.Duration(req.DurationMinute) * time.Minute),
	})
	if err != nil {
		code, msg := h.handleError("创建静音失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsAlertSilenceResp(sil))
}

// DeleteSilence 删除静音。
func (h *OpsHandler) DeleteSilence(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的静音 ID")
		return
	}
	if err := h.service.DeleteSilence(c.Request.Context(), id); err != nil {
		code, msg := h.handleError("删除静音失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func toAlertRuleInput(req dto.OpsAlertRuleReq) appops.AlertRuleInput {
	return appops.AlertRuleInput{
		Name:            req.Name,
		Metric:          req.Metric,
		Operator:        req.Operator,
		Threshold:       req.Threshold,
		WindowSeconds:   req.WindowSeconds,
		Severity:        req.Severity,
		Enabled:         req.Enabled,
		CooldownSeconds: req.CooldownSeconds,
		NotifyEmail:     req.NotifyEmail,
		Platform:        req.Platform,
	}
}
