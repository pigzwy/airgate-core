package ops

import (
	"context"
	"testing"
)

func TestHealth_HealthyWhenClean(t *testing.T) {
	repo := newFakeRepo()
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{
			Summary: AnalyticsSummary{TotalRequests: 100, ErrorRate: 0},
			Latency: Percentiles{P95: 200, Samples: 100},
		}, nil
	}
	svc := newTestService(repo)
	h, err := svc.Health(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if h.Score != 100 || h.Grade != "healthy" {
		t.Errorf("干净状态应满分 healthy，实际 %d/%s", h.Score, h.Grade)
	}
	if len(h.Diagnostics) != 1 || h.Diagnostics[0].Level != "info" {
		t.Errorf("应只有一条 info 诊断，实际 %+v", h.Diagnostics)
	}
}

func TestHealth_CriticalErrorRateDeducts(t *testing.T) {
	repo := newFakeRepo()
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{
			Summary: AnalyticsSummary{TotalRequests: 100, ErrorRate: 0.2}, // > crit 0.08
			Latency: Percentiles{P95: 12000, Samples: 100},                // > p95 crit 8000
		}, nil
	}
	svc := newTestService(repo)
	h, _ := svc.Health(context.Background())
	if h.Score >= 100 {
		t.Errorf("应扣分，实际 %d", h.Score)
	}
	if h.BusinessSub < 40 {
		t.Errorf("业务扣分应 >=40（错误率严重 40），实际 %d", h.BusinessSub)
	}
	// 应包含 critical 诊断
	hasCrit := false
	for _, d := range h.Diagnostics {
		if d.Level == "critical" {
			hasCrit = true
		}
	}
	if !hasCrit {
		t.Error("应含 critical 诊断")
	}
	// 分级应为 warning 或 critical（非 healthy）
	if h.Grade == "healthy" {
		t.Errorf("严重错误率不应判 healthy")
	}
}

func TestHealth_ScoreClampedNonNegative(t *testing.T) {
	repo := newFakeRepo()
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		return Analytics{
			Summary: AnalyticsSummary{TotalRequests: 100, ErrorRate: 1},
			Latency: Percentiles{P95: 99999, Samples: 100},
			TTFT:    Percentiles{P95: 99999, Samples: 100},
		}, nil
	}
	svc := newTestService(repo)
	h, _ := svc.Health(context.Background())
	if h.Score < 0 {
		t.Errorf("分数不应为负，实际 %d", h.Score)
	}
	if h.Grade != "critical" {
		t.Errorf("应为 critical，实际 %s", h.Grade)
	}
}
