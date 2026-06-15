package ops

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListRequestLogs_DefaultsAndValidation(t *testing.T) {
	repo := newFakeRepo()
	var captured RequestLogFilter
	repo.listRequestLogsFn = func(ctx context.Context, f RequestLogFilter) (RequestLogResult, error) {
		captured = f
		return RequestLogResult{}, nil
	}
	svc := newTestService(repo)

	// 非法页码/页大小应被纠正
	if _, err := svc.ListRequestLogs(context.Background(), RequestLogFilter{Page: 0, PageSize: 0}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if captured.Page != 1 || captured.PageSize != 50 {
		t.Errorf("默认分页应 1/50，实际 %d/%d", captured.Page, captured.PageSize)
	}

	// PageSize 超上限纠正
	_, _ = svc.ListRequestLogs(context.Background(), RequestLogFilter{Page: 2, PageSize: 9999})
	if captured.PageSize != 50 {
		t.Errorf("超限页大小应回 50，实际 %d", captured.PageSize)
	}

	// 时间区间倒置应报错
	start := time.Now()
	end := start.Add(-time.Hour)
	if _, err := svc.ListRequestLogs(context.Background(), RequestLogFilter{Start: &start, End: &end}); !errors.Is(err, ErrInvalidTimeRange) {
		t.Errorf("倒置区间应返回 ErrInvalidTimeRange，实际 %v", err)
	}
}

func TestAnalytics_RangeClamp(t *testing.T) {
	repo := newFakeRepo()
	var captured AnalyticsFilter
	repo.queryAnalyticsFn = func(ctx context.Context, f AnalyticsFilter) (Analytics, error) {
		captured = f
		return Analytics{}, nil
	}
	svc := newTestService(repo)

	// 0 → 默认 1 小时
	_, _ = svc.Analytics(context.Background(), 0, "")
	if d := captured.End.Sub(captured.Start); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("默认应约 1 小时，实际 %v", d)
	}

	// 超 7 天 → 截断 7 天
	_, _ = svc.Analytics(context.Background(), 999*24*3600, "openai")
	if d := captured.End.Sub(captured.Start); d > 7*24*time.Hour+time.Minute {
		t.Errorf("应截断到 7 天，实际 %v", d)
	}
	if captured.Platform != "openai" {
		t.Errorf("平台应透传，实际 %q", captured.Platform)
	}
}

func TestPurge_UsesRetention(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	now := time.Now()
	if err := svc.Purge(context.Background(), 24*time.Hour, 48*time.Hour, 12*time.Hour); err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo.purgedLogBefore == nil || repo.purgedStatBefore == nil || repo.purgedSysLogBefore == nil {
		t.Fatal("三类清理都应被调用")
	}
	// 日志保留 24h → before ≈ now-24h
	if d := now.Sub(*repo.purgedLogBefore); d < 23*time.Hour || d > 25*time.Hour {
		t.Errorf("日志清理时间点不对: %v", d)
	}
}

func TestSwitchRateTrend_NilRedisReturnsEmpty(t *testing.T) {
	svc := newTestService(newFakeRepo()) // rdb 未注入
	got, err := svc.SwitchRateTrend(context.Background(), 60)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Errorf("无 redis 应返回 nil，实际 %v", got)
	}
}
