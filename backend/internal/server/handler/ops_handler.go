package handler

import (
	"errors"
	"log/slog"
	"net/http"

	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
)

// OpsHandler 运维监控 Handler（管理员）。
type OpsHandler struct {
	service *appops.Service
}

// NewOpsHandler 创建 OpsHandler。
func NewOpsHandler(service *appops.Service) *OpsHandler {
	return &OpsHandler{service: service}
}

// handleError 将 ops 域哨兵错误映射为 HTTP code + 对外消息。
func (h *OpsHandler) handleError(logMessage string, err error) (int, string) {
	slog.Error(logMessage, "error", err)
	switch {
	case errors.Is(err, appops.ErrRequestLogNotFound):
		return http.StatusNotFound, "请求日志不存在"
	case errors.Is(err, appops.ErrInvalidTimeRange):
		return http.StatusBadRequest, "时间范围无效"
	default:
		return http.StatusInternalServerError, "查询失败"
	}
}
