package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// Overview 返回实时大盘概览（最近窗口汇总 + 趋势）。
func (h *OpsHandler) Overview(c *gin.Context) {
	var req dto.OpsOverviewReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}

	overview, err := h.service.Overview(c.Request.Context(), req.TrendWindows)
	if err != nil {
		code, msg := h.handleError("查询运维大盘失败", err)
		response.Error(c, code, -1, msg)
		return
	}

	response.Success(c, toOpsOverviewResp(overview))
}

// ErrorLogs 分页返回错误/请求日志。
func (h *OpsHandler) ErrorLogs(c *gin.Context) {
	var req dto.OpsErrorLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}

	filter := appops.RequestLogFilter{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Platform:  req.Platform,
		Model:     req.Model,
		ErrorKind: req.ErrorKind,
		// 默认只看错误；显式传 only_errors=0 才看全部。
		OnlyErrors: req.OnlyErrors == nil || *req.OnlyErrors,
	}
	if t, ok := parseRFC3339(req.Start); ok {
		filter.Start = &t
	}
	if t, ok := parseRFC3339(req.End); ok {
		filter.End = &t
	}

	result, err := h.service.ListRequestLogs(c.Request.Context(), filter)
	if err != nil {
		code, msg := h.handleError("查询运维错误日志失败", err)
		response.Error(c, code, -1, msg)
		return
	}

	response.Success(c, response.PagedData(
		toOpsRequestLogResps(result.List),
		result.Total,
		result.Page,
		result.PageSize,
	))
}

// ErrorDetail 返回单条请求日志详情（钻取）。
func (h *OpsHandler) ErrorDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的日志 ID")
		return
	}

	log, err := h.service.GetRequestLog(c.Request.Context(), id)
	if err != nil {
		code, msg := h.handleError("查询运维错误详情失败", err)
		response.Error(c, code, -1, msg)
		return
	}

	response.Success(c, toOpsRequestLogResp(log))
}

// parseRFC3339 解析 RFC3339 时间字符串；空串或解析失败返回 ok=false。
func parseRFC3339(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
