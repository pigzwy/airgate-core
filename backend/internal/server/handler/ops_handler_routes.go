package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// SystemMetrics 返回 Core 进程的系统资源快照（Goroutines/内存/Redis/后台任务）。
func (h *OpsHandler) SystemMetrics(c *gin.Context) {
	m := h.service.SystemMetrics(c.Request.Context())
	response.Success(c, toOpsSystemMetricsResp(m))
}

// Overview 返回实时大盘概览（最近窗口汇总 + 趋势）。
func (h *OpsHandler) Overview(c *gin.Context) {
	var req dto.OpsOverviewReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}

	overview, err := h.service.Overview(c.Request.Context(), req.TrendWindows, req.Platform)
	if err != nil {
		code, msg := h.handleError("查询运维大盘失败", err)
		response.Error(c, code, -1, msg)
		return
	}

	response.Success(c, toOpsOverviewResp(overview))
}

// streamInterval SSE 推送间隔。
const streamInterval = 2 * time.Second

// Stream 以 SSE 推送实时大盘快照（M5）。鉴权走 adminGroup 的 header JWT，
// 前端用 fetch 流式读取（非原生 EventSource，以便携带 Authorization 头）。
func (h *OpsHandler) Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	ctx := c.Request.Context()
	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		response.Error(c, 500, -1, "streaming unsupported")
		return
	}

	write := func() bool {
		snap := h.service.LiveSnapshot(ctx)
		b, err := json.Marshal(snap)
		if err != nil {
			return true
		}
		if _, err := c.Writer.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// 立即推一帧，随后按间隔推送。
	if !write() {
		return
	}
	ticker := time.NewTicker(streamInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !write() {
				return
			}
		}
	}
}

// SwitchRate 返回最近 N 分钟的账号切换率趋势（M13）。
func (h *OpsHandler) SwitchRate(c *gin.Context) {
	minutes, _ := strconv.Atoi(c.Query("minutes"))
	points, err := h.service.SwitchRateTrend(c.Request.Context(), minutes)
	if err != nil {
		code, msg := h.handleError("查询切换率失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	out := make([]dto.OpsSwitchRatePointResp, 0, len(points))
	for _, p := range points {
		out = append(out, dto.OpsSwitchRatePointResp{
			Minute:     p.Minute.In(beijingTZ).Format(time.RFC3339),
			Sets:       p.Sets,
			Switches:   p.Switches,
			SwitchRate: p.SwitchRate,
		})
	}
	response.Success(c, out)
}

// Health 返回健康分数 + 诊断报告（M4）。
func (h *OpsHandler) Health(c *gin.Context) {
	result, err := h.service.Health(c.Request.Context())
	if err != nil {
		code, msg := h.handleError("查询健康分数失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsHealthResp(result))
}

// SystemLogs 分页返回网关系统日志（M11）。
func (h *OpsHandler) SystemLogs(c *gin.Context) {
	var req dto.OpsSystemLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}
	filter := appops.SystemLogFilter{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Level:     req.Level,
		Component: req.Component,
		RequestID: req.RequestID,
		Keyword:   req.Keyword,
	}
	if t, ok := parseRFC3339(req.Start); ok {
		filter.Start = &t
	}
	if t, ok := parseRFC3339(req.End); ok {
		filter.End = &t
	}
	result, err := h.service.ListSystemLogs(c.Request.Context(), filter)
	if err != nil {
		code, msg := h.handleError("查询系统日志失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, response.PagedData(
		toOpsSystemLogResps(result.List),
		result.Total,
		result.Page,
		result.PageSize,
	))
}

// GetLogLevel 返回当前/默认日志级别。
func (h *OpsHandler) GetLogLevel(c *gin.Context) {
	cur, def, dropped := h.service.LogLevel()
	response.Success(c, dto.OpsLogLevelResp{Level: cur, Default: def, Dropped: dropped})
}

// SetLogLevel 运行时调整日志级别。
func (h *OpsHandler) SetLogLevel(c *gin.Context) {
	var req dto.OpsLogLevelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	if err := h.service.SetLogLevel(req.Level); err != nil {
		code, msg := h.handleError("设置日志级别失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	cur, def, dropped := h.service.LogLevel()
	response.Success(c, dto.OpsLogLevelResp{Level: cur, Default: def, Dropped: dropped})
}

// ResetLogLevel 恢复默认日志级别。
func (h *OpsHandler) ResetLogLevel(c *gin.Context) {
	h.service.ResetLogLevel()
	cur, def, dropped := h.service.LogLevel()
	response.Success(c, dto.OpsLogLevelResp{Level: cur, Default: def, Dropped: dropped})
}

// Concurrency 返回当前并发统计（账号/平台/分组三维 + 账号可用性）。
func (h *OpsHandler) Concurrency(c *gin.Context) {
	result, err := h.service.Concurrency(c.Request.Context())
	if err != nil {
		code, msg := h.handleError("查询并发统计失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsConcurrencyResp(result))
}

// Analytics 按时间范围返回全套分析指标（全分位/TTFT/直方图/错误分布/Token/平台维度）。
func (h *OpsHandler) Analytics(c *gin.Context) {
	var req dto.OpsAnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}

	result, err := h.service.Analytics(c.Request.Context(), req.RangeSeconds, req.Platform)
	if err != nil {
		code, msg := h.handleError("查询运维分析失败", err)
		response.Error(c, code, -1, msg)
		return
	}

	response.Success(c, toOpsAnalyticsResp(result))
}

// ErrorLogs 分页返回错误/请求日志。
func (h *OpsHandler) ErrorLogs(c *gin.Context) {
	var req dto.OpsErrorLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BindError(c, err)
		return
	}

	filter := appops.RequestLogFilter{
		Page:          req.Page,
		PageSize:      req.PageSize,
		Platform:      req.Platform,
		Model:         req.Model,
		ErrorKind:     req.ErrorKind,
		SuccessMode:   req.SuccessMode,
		MinDurationMs: req.MinDurationMs,
		SortBy:        req.SortBy,
		// 默认降序；显式传 sort_desc=0 才升序。
		SortDesc: req.SortDesc == nil || *req.SortDesc,
		// 默认只看错误；显式传 only_errors=0 或 success_mode 才看全部。
		OnlyErrors: req.SuccessMode == "" && (req.OnlyErrors == nil || *req.OnlyErrors),
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

// Trace 返回一次请求的重试链路（同 client_request_id 的所有日志，M12）。
func (h *OpsHandler) Trace(c *gin.Context) {
	cid := c.Query("client_request_id")
	if cid == "" {
		response.BadRequest(c, "缺少 client_request_id")
		return
	}
	logs, err := h.service.Trace(c.Request.Context(), cid)
	if err != nil {
		code, msg := h.handleError("查询请求链路失败", err)
		response.Error(c, code, -1, msg)
		return
	}
	response.Success(c, toOpsRequestLogResps(logs))
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
