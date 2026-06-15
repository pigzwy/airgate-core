package ops

import (
	"context"
	"sort"
	"time"
)

// ===== 并发统计（M7）：scheduler 当前并发 + 账号可用性 =====

// AccountConcurrencyInfo 账号的并发配置与状态（由 Repository 从 DB 读取）。
type AccountConcurrencyInfo struct {
	AccountID      int
	Name           string
	Platform       string
	Groups         []string
	MaxConcurrency int
	State          string // active / rate_limited / degraded / disabled
	StateUntil     *time.Time
}

// ConcurrencyCounter 当前并发计数来源（由 scheduler.ConcurrencyManager 实现）。
// 仅声明所需方法，避免 ops 直接耦合 scheduler 具体类型。
type ConcurrencyCounter interface {
	GetCurrentCounts(ctx context.Context, accountIDs []int) map[int]int
}

// ConcurrencyItem 一个维度（平台/分组）的并发汇总。
type ConcurrencyItem struct {
	Key     string  `json:"key"`
	Current int     `json:"current"`
	Max     int     `json:"max"`
	Usage   float64 `json:"usage"` // current/max，max=0 时为 0
}

// AccountConcurrencyDetail 单账号并发明细。
type AccountConcurrencyDetail struct {
	AccountID       int     `json:"account_id"`
	Name            string  `json:"name"`
	Platform        string  `json:"platform"`
	Current         int     `json:"current"`
	Max             int     `json:"max"`
	Usage           float64 `json:"usage"`
	State           string  `json:"state"`
	RecoverySeconds int64   `json:"recovery_seconds"` // rate_limited/degraded 距恢复秒数，0=不适用
}

// AccountAvailability 账号可用性汇总（按状态计数 + 最近恢复倒计时）。
type AccountAvailability struct {
	Active                 int   `json:"active"`
	RateLimited            int   `json:"rate_limited"`
	Degraded               int   `json:"degraded"`
	Disabled               int   `json:"disabled"`
	Total                  int   `json:"total"`
	SoonestRecoverySeconds int64 `json:"soonest_recovery_seconds"` // 0=无待恢复账号
}

// Concurrency 并发统计完整结果。
type Concurrency struct {
	Availability AccountAvailability
	ByAccount    []AccountConcurrencyDetail
	ByPlatform   []ConcurrencyItem
	ByGroup      []ConcurrencyItem
	TotalCurrent int
	TotalMax     int
}

// SetConcurrencyCounter 注入当前并发计数器。装配期调用一次。
func (s *Service) SetConcurrencyCounter(c ConcurrencyCounter) {
	s.counter = c
}

// Concurrency 汇总当前并发（账号/平台/分组三维 + 账号可用性）。
//
// 注：用户维度需 scheduler 暴露用户槽位批量计数接口（当前未提供），暂缺。
func (s *Service) Concurrency(ctx context.Context) (Concurrency, error) {
	infos, err := s.repo.ListAccountConcurrency(ctx)
	if err != nil {
		return Concurrency{}, err
	}

	out := Concurrency{}
	now := time.Now()

	// 当前并发计数（Redis）。counter 未注入时全部按 0 处理（降级）。
	ids := make([]int, 0, len(infos))
	for _, a := range infos {
		ids = append(ids, a.AccountID)
	}
	var counts map[int]int
	if s.counter != nil && len(ids) > 0 {
		counts = s.counter.GetCurrentCounts(ctx, ids)
	} else {
		counts = map[int]int{}
	}

	platAgg := map[string]*ConcurrencyItem{}
	groupAgg := map[string]*ConcurrencyItem{}

	for _, a := range infos {
		cur := counts[a.AccountID]
		out.TotalCurrent += cur
		out.TotalMax += a.MaxConcurrency

		// 可用性计数
		out.Availability.Total++
		switch a.State {
		case "active":
			out.Availability.Active++
		case "rate_limited":
			out.Availability.RateLimited++
		case "degraded":
			out.Availability.Degraded++
		case "disabled":
			out.Availability.Disabled++
		}

		// 恢复倒计时（仅 rate_limited / degraded 有 state_until）
		var recovery int64
		if (a.State == "rate_limited" || a.State == "degraded") && a.StateUntil != nil {
			if d := a.StateUntil.Sub(now); d > 0 {
				recovery = int64(d.Seconds())
				if out.Availability.SoonestRecoverySeconds == 0 || recovery < out.Availability.SoonestRecoverySeconds {
					out.Availability.SoonestRecoverySeconds = recovery
				}
			}
		}

		detail := AccountConcurrencyDetail{
			AccountID:       a.AccountID,
			Name:            a.Name,
			Platform:        a.Platform,
			Current:         cur,
			Max:             a.MaxConcurrency,
			Usage:           usageRatio(cur, a.MaxConcurrency),
			State:           a.State,
			RecoverySeconds: recovery,
		}
		out.ByAccount = append(out.ByAccount, detail)

		// 平台维度
		p, ok := platAgg[a.Platform]
		if !ok {
			p = &ConcurrencyItem{Key: a.Platform}
			platAgg[a.Platform] = p
		}
		p.Current += cur
		p.Max += a.MaxConcurrency

		// 分组维度（账号可属多组，按每组累加）
		for _, g := range a.Groups {
			gi, ok := groupAgg[g]
			if !ok {
				gi = &ConcurrencyItem{Key: g}
				groupAgg[g] = gi
			}
			gi.Current += cur
			gi.Max += a.MaxConcurrency
		}
	}

	// 账号明细按当前并发降序（热点在前）
	sort.Slice(out.ByAccount, func(i, j int) bool {
		if out.ByAccount[i].Current != out.ByAccount[j].Current {
			return out.ByAccount[i].Current > out.ByAccount[j].Current
		}
		return out.ByAccount[i].AccountID < out.ByAccount[j].AccountID
	})

	out.ByPlatform = finalizeItems(platAgg)
	out.ByGroup = finalizeItems(groupAgg)
	return out, nil
}

// usageRatio 计算使用率，max<=0 返回 0。
func usageRatio(current, max int) float64 {
	if max <= 0 {
		return 0
	}
	return float64(current) / float64(max)
}

// finalizeItems 把聚合 map 转为按使用率降序的切片，并补齐 Usage。
func finalizeItems(m map[string]*ConcurrencyItem) []ConcurrencyItem {
	out := make([]ConcurrencyItem, 0, len(m))
	for _, it := range m {
		it.Usage = usageRatio(it.Current, it.Max)
		out = append(out, *it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current > out[j].Current
		}
		return out[i].Key < out[j].Key
	})
	return out
}
