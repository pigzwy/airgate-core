package ops

import (
	"context"
	"time"
)

// ===== 实时推送（M5）：SSE 秒级快照 =====
//
// 用 SSE（Server-Sent Events）而非 WebSocket：零新增依赖、走现有 header 鉴权、
// 反代友好，且大盘只需服务端单向推送，SSE 完全够用。

// StreamSnapshot 实时大盘快照（每 ~2s 推送一次）。
type StreamSnapshot struct {
	RPS           float64   `json:"rps"`
	ErrorRate     float64   `json:"error_rate"`
	P95DurationMs int64     `json:"p95_duration_ms"`
	TotalRequests int64     `json:"total_requests"`
	Goroutines    int       `json:"goroutines"`
	CapturedAt    time.Time `json:"captured_at"`
}

// LiveSnapshot 计算一份实时快照（最近 60s 分析 + 系统指标）。
func (s *Service) LiveSnapshot(ctx context.Context) StreamSnapshot {
	out := StreamSnapshot{CapturedAt: time.Now()}
	a, err := s.repo.QueryAnalytics(ctx, AnalyticsFilter{
		Start: time.Now().Add(-60 * time.Second),
		End:   time.Now(),
	})
	if err == nil {
		out.RPS = a.Summary.RPS
		out.ErrorRate = a.Summary.ErrorRate
		out.P95DurationMs = a.Latency.P95
		out.TotalRequests = a.Summary.TotalRequests
	}
	out.Goroutines = s.SystemMetrics(ctx).Goroutines
	return out
}
