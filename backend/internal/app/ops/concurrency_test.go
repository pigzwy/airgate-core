package ops

import (
	"context"
	"testing"
	"time"
)

func TestUsageRatio(t *testing.T) {
	if usageRatio(5, 0) != 0 {
		t.Error("max=0 应返回 0")
	}
	if usageRatio(3, 6) != 0.5 {
		t.Error("3/6 应为 0.5")
	}
}

func TestFinalizeItems_SortAndUsage(t *testing.T) {
	m := map[string]*ConcurrencyItem{
		"a": {Key: "a", Current: 1, Max: 10},
		"b": {Key: "b", Current: 5, Max: 10},
	}
	out := finalizeItems(m)
	if len(out) != 2 || out[0].Key != "b" {
		t.Fatalf("应按 current 降序，b 在前: %+v", out)
	}
	if out[0].Usage != 0.5 {
		t.Errorf("b usage 应 0.5，实际 %v", out[0].Usage)
	}
}

// fakeCounter 实现 ConcurrencyCounter。
type fakeCounter map[int]int

func (f fakeCounter) GetCurrentCounts(ctx context.Context, ids []int) map[int]int {
	out := map[int]int{}
	for _, id := range ids {
		out[id] = f[id]
	}
	return out
}

func TestConcurrency_AggregatesAndAvailability(t *testing.T) {
	repo := newFakeRepo()
	until := time.Now().Add(2 * time.Minute)
	repo.listAccountConcurrencyFn = func(ctx context.Context) ([]AccountConcurrencyInfo, error) {
		return []AccountConcurrencyInfo{
			{AccountID: 1, Name: "a1", Platform: "openai", Groups: []string{"g1"}, MaxConcurrency: 10, State: "active"},
			{AccountID: 2, Name: "a2", Platform: "openai", Groups: []string{"g1", "g2"}, MaxConcurrency: 10, State: "rate_limited", StateUntil: &until},
			{AccountID: 3, Name: "a3", Platform: "claude", Groups: nil, MaxConcurrency: 5, State: "disabled"},
		}, nil
	}
	svc := newTestService(repo)
	svc.SetConcurrencyCounter(fakeCounter{1: 3, 2: 7, 3: 0})

	got, err := svc.Concurrency(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.TotalCurrent != 10 || got.TotalMax != 25 {
		t.Errorf("总并发 %d/%d，期望 10/25", got.TotalCurrent, got.TotalMax)
	}
	if got.Availability.Active != 1 || got.Availability.RateLimited != 1 || got.Availability.Disabled != 1 {
		t.Errorf("可用性计数不对: %+v", got.Availability)
	}
	if got.Availability.SoonestRecoverySeconds <= 0 {
		t.Error("应有恢复倒计时")
	}
	// 平台维度：openai current=10/max=20
	var openai *ConcurrencyItem
	for i := range got.ByPlatform {
		if got.ByPlatform[i].Key == "openai" {
			openai = &got.ByPlatform[i]
		}
	}
	if openai == nil || openai.Current != 10 || openai.Max != 20 {
		t.Errorf("openai 平台聚合不对: %+v", openai)
	}
	// 分组维度：g1 含账号1+2 → current=10
	var g1 *ConcurrencyItem
	for i := range got.ByGroup {
		if got.ByGroup[i].Key == "g1" {
			g1 = &got.ByGroup[i]
		}
	}
	if g1 == nil || g1.Current != 10 {
		t.Errorf("g1 分组聚合不对: %+v", g1)
	}
}
