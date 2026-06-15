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

func toOpsSystemMetricsResp(m appops.SystemMetrics) dto.OpsSystemMetricsResp {
	jobs := make([]dto.OpsJobResp, 0, len(m.Jobs))
	for _, j := range m.Jobs {
		jr := dto.OpsJobResp{
			Name:      j.Name,
			Running:   j.Running,
			LastError: j.LastError,
		}
		if j.LastHeartbeat != nil && !j.LastHeartbeat.IsZero() {
			jr.LastHeartbeat = j.LastHeartbeat.In(beijingTZ).Format(time.RFC3339)
		}
		if j.LastRunAt != nil && !j.LastRunAt.IsZero() {
			jr.LastRunAt = j.LastRunAt.In(beijingTZ).Format(time.RFC3339)
		}
		jobs = append(jobs, jr)
	}
	out := dto.OpsSystemMetricsResp{
		Goroutines: m.Goroutines,
		Memory: dto.OpsMemoryResp{
			HeapAllocBytes: m.Memory.HeapAllocBytes,
			SysBytes:       m.Memory.SysBytes,
			NumGC:          m.Memory.NumGC,
		},
		DB: dto.OpsDBResp{
			MaxOpenConns:      m.DB.MaxOpenConns,
			OpenConns:         m.DB.OpenConns,
			InUse:             m.DB.InUse,
			Idle:              m.DB.Idle,
			WaitCount:         m.DB.WaitCount,
			WaitDurationMs:    m.DB.WaitDurationMs,
			MaxIdleClosed:     m.DB.MaxIdleClosed,
			MaxLifetimeClosed: m.DB.MaxLifetimeClosed,
		},
		Redis: dto.OpsRedisResp{
			Hits:       m.Redis.Hits,
			Misses:     m.Redis.Misses,
			Timeouts:   m.Redis.Timeouts,
			TotalConns: m.Redis.TotalConns,
			IdleConns:  m.Redis.IdleConns,
			StaleConns: m.Redis.StaleConns,
		},
		Jobs: jobs,
	}
	if !m.CapturedAt.IsZero() {
		out.CapturedAt = m.CapturedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsPercentilesResp(p appops.Percentiles) dto.OpsPercentilesResp {
	return dto.OpsPercentilesResp{
		P50:     p.P50,
		P90:     p.P90,
		P95:     p.P95,
		P99:     p.P99,
		Max:     p.Max,
		Avg:     p.Avg,
		Samples: p.Samples,
	}
}

func toOpsAnalyticsResp(a appops.Analytics) dto.OpsAnalyticsResp {
	hist := make([]dto.OpsHistogramBucketResp, 0, len(a.Histogram))
	for _, b := range a.Histogram {
		hist = append(hist, dto.OpsHistogramBucketResp{
			Label: b.Label,
			MinMs: b.Min,
			MaxMs: b.Max,
			Count: b.Count,
		})
	}
	errs := make([]dto.OpsErrorClassResp, 0, len(a.ErrorDistribution))
	for _, e := range a.ErrorDistribution {
		errs = append(errs, dto.OpsErrorClassResp{
			Kind:  e.Kind,
			Label: e.Label,
			Count: e.Count,
			Ratio: e.Ratio,
		})
	}
	models := make([]dto.OpsModelTokenResp, 0, len(a.TokensByModel))
	for _, m := range a.TokensByModel {
		models = append(models, dto.OpsModelTokenResp{
			Model:        m.Model,
			Requests:     m.Requests,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
		})
	}
	plats := make([]dto.OpsPlatformStatResp, 0, len(a.PlatformBreakdown))
	for _, p := range a.PlatformBreakdown {
		plats = append(plats, dto.OpsPlatformStatResp{
			Platform:      p.Platform,
			Requests:      p.Requests,
			ErrorRequests: p.ErrorRequests,
			ErrorRate:     p.ErrorRate,
		})
	}
	return dto.OpsAnalyticsResp{
		Summary: dto.OpsAnalyticsSummaryResp{
			RangeSeconds:    a.Summary.RangeSeconds,
			TotalRequests:   a.Summary.TotalRequests,
			SuccessRequests: a.Summary.SuccessRequests,
			ErrorRequests:   a.Summary.ErrorRequests,
			ErrorRate:       a.Summary.ErrorRate,
			RPS:             a.Summary.RPS,
			Sampled:         a.Summary.Sampled,
		},
		Latency:           toOpsPercentilesResp(a.Latency),
		TTFT:              toOpsPercentilesResp(a.TTFT),
		Histogram:         hist,
		ErrorDistribution: errs,
		TokensByModel:     models,
		PlatformBreakdown: plats,
	}
}

func toOpsConcurrencyResp(c appops.Concurrency) dto.OpsConcurrencyResp {
	byAccount := make([]dto.OpsAccountConcurrencyResp, 0, len(c.ByAccount))
	for _, a := range c.ByAccount {
		byAccount = append(byAccount, dto.OpsAccountConcurrencyResp{
			AccountID:       a.AccountID,
			Name:            a.Name,
			Platform:        a.Platform,
			Current:         a.Current,
			Max:             a.Max,
			Usage:           a.Usage,
			State:           a.State,
			RecoverySeconds: a.RecoverySeconds,
		})
	}
	conv := func(items []appops.ConcurrencyItem) []dto.OpsConcurrencyItemResp {
		out := make([]dto.OpsConcurrencyItemResp, 0, len(items))
		for _, it := range items {
			out = append(out, dto.OpsConcurrencyItemResp{
				Key:     it.Key,
				Current: it.Current,
				Max:     it.Max,
				Usage:   it.Usage,
			})
		}
		return out
	}
	return dto.OpsConcurrencyResp{
		Availability: dto.OpsAccountAvailabilityResp{
			Active:                 c.Availability.Active,
			RateLimited:            c.Availability.RateLimited,
			Degraded:               c.Availability.Degraded,
			Disabled:               c.Availability.Disabled,
			Total:                  c.Availability.Total,
			SoonestRecoverySeconds: c.Availability.SoonestRecoverySeconds,
		},
		ByAccount:    byAccount,
		ByPlatform:   conv(c.ByPlatform),
		ByGroup:      conv(c.ByGroup),
		TotalCurrent: c.TotalCurrent,
		TotalMax:     c.TotalMax,
	}
}

func toOpsAlertRuleResp(r appops.AlertRule) dto.OpsAlertRuleResp {
	out := dto.OpsAlertRuleResp{
		ID:              r.ID,
		Name:            r.Name,
		Metric:          r.Metric,
		Operator:        r.Operator,
		Threshold:       r.Threshold,
		WindowSeconds:   r.WindowSeconds,
		Severity:        r.Severity,
		Enabled:         r.Enabled,
		CooldownSeconds: r.CooldownSeconds,
		NotifyEmail:     r.NotifyEmail,
		Platform:        r.Platform,
	}
	if r.LastFiredAt != nil && !r.LastFiredAt.IsZero() {
		out.LastFiredAt = r.LastFiredAt.In(beijingTZ).Format(time.RFC3339)
	}
	if !r.CreatedAt.IsZero() {
		out.CreatedAt = r.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsAlertRuleResps(items []appops.AlertRule) []dto.OpsAlertRuleResp {
	out := make([]dto.OpsAlertRuleResp, 0, len(items))
	for _, r := range items {
		out = append(out, toOpsAlertRuleResp(r))
	}
	return out
}

func toOpsAlertEventResp(e appops.AlertEvent) dto.OpsAlertEventResp {
	out := dto.OpsAlertEventResp{
		ID:        e.ID,
		RuleID:    e.RuleID,
		RuleName:  e.RuleName,
		Metric:    e.Metric,
		Operator:  e.Operator,
		Value:     e.Value,
		Threshold: e.Threshold,
		Severity:  e.Severity,
		Status:    e.Status,
		Message:   e.Message,
	}
	if !e.CreatedAt.IsZero() {
		out.CreatedAt = e.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	if e.ResolvedAt != nil && !e.ResolvedAt.IsZero() {
		out.ResolvedAt = e.ResolvedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsAlertEventResps(items []appops.AlertEvent) []dto.OpsAlertEventResp {
	out := make([]dto.OpsAlertEventResp, 0, len(items))
	for _, e := range items {
		out = append(out, toOpsAlertEventResp(e))
	}
	return out
}

func toOpsAlertSilenceResp(s appops.AlertSilence) dto.OpsAlertSilenceResp {
	out := dto.OpsAlertSilenceResp{
		ID:     s.ID,
		RuleID: s.RuleID,
		Reason: s.Reason,
	}
	if !s.Until.IsZero() {
		out.Until = s.Until.In(beijingTZ).Format(time.RFC3339)
	}
	if !s.CreatedAt.IsZero() {
		out.CreatedAt = s.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsAlertSilenceResps(items []appops.AlertSilence) []dto.OpsAlertSilenceResp {
	out := make([]dto.OpsAlertSilenceResp, 0, len(items))
	for _, s := range items {
		out = append(out, toOpsAlertSilenceResp(s))
	}
	return out
}

func toOpsHealthResp(h appops.HealthScore) dto.OpsHealthResp {
	items := make([]dto.OpsDiagnosticResp, 0, len(h.Diagnostics))
	for _, d := range h.Diagnostics {
		items = append(items, dto.OpsDiagnosticResp{
			Level:      d.Level,
			Title:      d.Title,
			Detail:     d.Detail,
			Suggestion: d.Suggestion,
		})
	}
	out := dto.OpsHealthResp{
		Score:       h.Score,
		Grade:       h.Grade,
		BusinessSub: h.BusinessSub,
		InfraSub:    h.InfraSub,
		Diagnostics: items,
	}
	if !h.CapturedAt.IsZero() {
		out.CapturedAt = h.CapturedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsSystemLogResp(e appops.SystemLogEntry) dto.OpsSystemLogResp {
	out := dto.OpsSystemLogResp{
		ID:        e.ID,
		Level:     e.Level,
		Component: e.Component,
		Message:   e.Message,
		RequestID: e.RequestID,
		Attrs:     e.Attrs,
	}
	if !e.CreatedAt.IsZero() {
		out.CreatedAt = e.CreatedAt.In(beijingTZ).Format(time.RFC3339)
	}
	return out
}

func toOpsSystemLogResps(items []appops.SystemLogEntry) []dto.OpsSystemLogResp {
	out := make([]dto.OpsSystemLogResp, 0, len(items))
	for _, e := range items {
		out = append(out, toOpsSystemLogResp(e))
	}
	return out
}

func toOpsRequestLogResp(r appops.RequestLog) dto.OpsRequestLogResp {
	out := dto.OpsRequestLogResp{
		ID:                 r.ID,
		RequestID:          r.RequestID,
		ClientRequestID:    r.ClientRequestID,
		PluginID:           r.PluginID,
		Platform:           r.Platform,
		Model:              r.Model,
		UpstreamModel:      r.UpstreamModel,
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
