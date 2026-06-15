package ops

import (
	"context"
	"fmt"
	"time"
)

// ===== 健康分数 + 诊断（M4）=====

// DiagnosticItem 单条诊断项。
type DiagnosticItem struct {
	Level      string `json:"level"`      // critical / warning / info
	Title      string `json:"title"`      // 问题摘要
	Detail     string `json:"detail"`     // 当前指标值描述
	Suggestion string `json:"suggestion"` // 处置建议
}

// HealthScore 健康分数 + 诊断报告。
type HealthScore struct {
	Score       int              `json:"score"`        // 0-100
	Grade       string           `json:"grade"`        // healthy / warning / critical
	BusinessSub int              `json:"business_sub"` // 业务层扣分合计
	InfraSub    int              `json:"infra_sub"`    // 基础设施层扣分合计
	Diagnostics []DiagnosticItem `json:"diagnostics"`
	CapturedAt  time.Time        `json:"captured_at"`
}

// 健康评分阈值（粗调，后续可在 M15 设置中心配置化）。
const (
	healthErrRateWarn   = 0.02 // 错误率 2% 起告警
	healthErrRateCrit   = 0.08 // 8% 起严重
	healthP95WarnMs     = 3000 // P95 延迟 3s 起告警
	healthP95CritMs     = 8000 // 8s 起严重
	healthTTFTWarnMs    = 3000
	healthDBUsageWarn   = 0.7
	healthDBUsageCrit   = 0.9
	healthGoroutineWarn = 2000
)

// Health 计算实时健康分数与诊断报告。
//
// 业务层（占比大）：错误率、P95 延迟、TTFT；基础设施层：DB 连接池、Redis 超时、Goroutines。
// 评分从 100 起扣，扣分项同时产出一条诊断。取最近 5 分钟分析窗口。
func (s *Service) Health(ctx context.Context) (HealthScore, error) {
	const recentWindow = 5 * 60 // 最近 5 分钟
	analytics, err := s.repo.QueryAnalytics(ctx, AnalyticsFilter{
		Start: time.Now().Add(-recentWindow * time.Second),
		End:   time.Now(),
	})
	if err != nil {
		return HealthScore{}, err
	}
	sys := s.SystemMetrics(ctx)

	out := HealthScore{Score: 100, Grade: "healthy", CapturedAt: time.Now()}
	add := func(level, title, detail, suggestion string, penalty int, business bool) {
		out.Score -= penalty
		if business {
			out.BusinessSub += penalty
		} else {
			out.InfraSub += penalty
		}
		out.Diagnostics = append(out.Diagnostics, DiagnosticItem{
			Level:      level,
			Title:      title,
			Detail:     detail,
			Suggestion: suggestion,
		})
	}

	// ---- 业务层 ----
	if analytics.Summary.TotalRequests > 0 {
		er := analytics.Summary.ErrorRate
		switch {
		case er >= healthErrRateCrit:
			add("critical", "错误率过高",
				fmt.Sprintf("最近 5 分钟错误率 %.2f%%（%d/%d）", er*100, analytics.Summary.ErrorRequests, analytics.Summary.TotalRequests),
				"检查上游账号可用性与错误分布，优先处理 upstream_5xx", 40, true)
		case er >= healthErrRateWarn:
			add("warning", "错误率偏高",
				fmt.Sprintf("最近 5 分钟错误率 %.2f%%", er*100),
				"关注错误分布饼图，确认是否集中在某平台/账号", 20, true)
		}
	}

	if analytics.Latency.Samples > 0 {
		p95 := analytics.Latency.P95
		switch {
		case p95 >= healthP95CritMs:
			add("critical", "延迟严重",
				fmt.Sprintf("P95 延迟 %d ms", p95),
				"排查上游响应慢或网关资源瓶颈，必要时扩容/切换账号", 20, true)
		case p95 >= healthP95WarnMs:
			add("warning", "延迟偏高",
				fmt.Sprintf("P95 延迟 %d ms", p95),
				"观察延迟趋势与直方图，确认是否持续恶化", 10, true)
		}
	}

	if analytics.TTFT.Samples > 0 && analytics.TTFT.P95 >= healthTTFTWarnMs {
		add("warning", "首 Token 延迟偏高",
			fmt.Sprintf("TTFT P95 %d ms", analytics.TTFT.P95),
			"流式首包慢影响体验，检查上游与路由选择", 10, true)
	}

	// ---- 基础设施层 ----
	if sys.DB.MaxOpenConns > 0 {
		usage := float64(sys.DB.InUse) / float64(sys.DB.MaxOpenConns)
		switch {
		case usage >= healthDBUsageCrit:
			add("critical", "数据库连接池接近耗尽",
				fmt.Sprintf("连接使用 %d/%d（%.0f%%）", sys.DB.InUse, sys.DB.MaxOpenConns, usage*100),
				"提高 max_open_conns 或排查慢查询/连接泄漏", 15, false)
		case usage >= healthDBUsageWarn:
			add("warning", "数据库连接池偏紧",
				fmt.Sprintf("连接使用 %d/%d", sys.DB.InUse, sys.DB.MaxOpenConns),
				"关注连接等待次数，必要时调大连接池", 8, false)
		}
	}
	if sys.DB.WaitCount > 0 {
		add("warning", "存在数据库连接等待",
			fmt.Sprintf("累计等待 %d 次", sys.DB.WaitCount),
			"连接池容量不足，考虑调大 max_open_conns", 5, false)
	}
	if sys.Redis.Timeouts > 0 {
		add("warning", "Redis 连接超时",
			fmt.Sprintf("累计超时 %d 次", sys.Redis.Timeouts),
			"检查 Redis 负载与网络，确认连接池配置", 5, false)
	}
	if sys.Goroutines > healthGoroutineWarn {
		add("warning", "Goroutine 数量偏高",
			fmt.Sprintf("当前 %d 个 Goroutine", sys.Goroutines),
			"可能存在 goroutine 泄漏，关注趋势并排查阻塞点", 5, false)
	}

	// 后台任务异常
	for _, j := range sys.Jobs {
		if j.LastError != "" {
			add("critical", "后台任务异常",
				fmt.Sprintf("任务 %s：%s", j.Name, j.LastError),
				"查看系统日志定位后台任务失败原因", 10, false)
		}
	}

	if out.Score < 0 {
		out.Score = 0
	}
	switch {
	case out.Score >= 80:
		out.Grade = "healthy"
	case out.Score >= 50:
		out.Grade = "warning"
	default:
		out.Grade = "critical"
	}

	if len(out.Diagnostics) == 0 {
		out.Diagnostics = append(out.Diagnostics, DiagnosticItem{
			Level:  "info",
			Title:  "运行正常",
			Detail: "未检测到异常指标",
		})
	}
	return out, nil
}
