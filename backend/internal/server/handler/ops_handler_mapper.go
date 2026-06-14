package handler

import (
	"time"

	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
	"github.com/DouDOU-start/airgate-core/internal/server/dto"
)

func toOpsWindowStatResp(w appops.WindowStat) dto.OpsWindowStatResp {
	out := dto.OpsWindowStatResp{
		WindowSeconds:         w.WindowSeconds,
		Platform:              w.Platform,
		TotalRequests:         w.TotalRequests,
		SuccessRequests:       w.SuccessRequests,
		ErrorRequests:         w.ErrorRequests,
		UpstreamErrorRequests: w.UpstreamErrorRequests,
		RPS:                   w.RPS,
		ErrorRate:             w.ErrorRate,
		P50DurationMs:         w.P50DurationMs,
		P95DurationMs:         w.P95DurationMs,
		P99DurationMs:         w.P99DurationMs,
		TotalInputTokens:      w.TotalInputTokens,
		TotalOutputTokens:     w.TotalOutputTokens,
	}
	if !w.WindowStart.IsZero() {
		out.WindowStart = w.WindowStart.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsOverviewResp(o appops.Overview) dto.OpsOverviewResp {
	trend := make([]dto.OpsWindowStatResp, 0, len(o.Trend))
	for _, w := range o.Trend {
		trend = append(trend, toOpsWindowStatResp(w))
	}
	return dto.OpsOverviewResp{
		Latest: toOpsWindowStatResp(o.Latest),
		Trend:  trend,
	}
}

func toOpsRequestLogResp(r appops.RequestLog) dto.OpsRequestLogResp {
	out := dto.OpsRequestLogResp{
		ID:                 r.ID,
		RequestID:          r.RequestID,
		PluginID:           r.PluginID,
		Platform:           r.Platform,
		Model:              r.Model,
		Endpoint:           r.Endpoint,
		UserID:             r.UserID,
		APIKeyID:           r.APIKeyID,
		AccountID:          r.AccountID,
		GroupID:            r.GroupID,
		Success:            r.Success,
		StatusCode:         r.StatusCode,
		UpstreamStatusCode: r.UpstreamStatusCode,
		DurationMs:         r.DurationMs,
		FirstTokenMs:       r.FirstTokenMs,
		Stream:             r.Stream,
		InputTokens:        r.InputTokens,
		OutputTokens:       r.OutputTokens,
		ErrorKind:          r.ErrorKind,
		ErrorMsg:           r.ErrorMsg,
		ErrorDetail:        r.ErrorDetail,
	}
	if !r.CreatedAt.IsZero() {
		out.CreatedAt = r.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsRequestLogResps(items []appops.RequestLog) []dto.OpsRequestLogResp {
	out := make([]dto.OpsRequestLogResp, 0, len(items))
	for _, r := range items {
		out = append(out, toOpsRequestLogResp(r))
	}
	return out
}
