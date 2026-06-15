package ops

import (
	"context"
	"strconv"
	"time"

	"github.com/DouDOU-start/airgate-core/internal/scheduler"
)

// ===== 切换率趋势（M13）：读取 scheduler 写入的分钟桶计数器 =====

// SwitchRatePoint 单分钟切换率数据点。
type SwitchRatePoint struct {
	Minute     time.Time `json:"minute"`
	Sets       int64     `json:"sets"`        // 粘性绑定次数
	Switches   int64     `json:"switches"`    // 账号切换次数
	SwitchRate float64   `json:"switch_rate"` // switches/sets
}

// SwitchRateTrend 返回最近 minutes 分钟的切换率趋势。rdb 未注入时返回空。
func (s *Service) SwitchRateTrend(ctx context.Context, minutes int) ([]SwitchRatePoint, error) {
	if s.rdb == nil {
		return nil, nil
	}
	if minutes <= 0 || minutes > 1440 {
		minutes = 60
	}
	nowMin := time.Now().Unix() / 60

	// 批量取计数器（pipeline）。
	pipe := s.rdb.Pipeline()
	setCmds := make([]interface{ Result() (string, error) }, minutes)
	swCmds := make([]interface{ Result() (string, error) }, minutes)
	for i := 0; i < minutes; i++ {
		m := nowMin - int64(minutes-1-i)
		setCmds[i] = pipe.Get(ctx, scheduler.StickySetKey(m))
		swCmds[i] = pipe.Get(ctx, scheduler.StickySwitchKey(m))
	}
	_, _ = pipe.Exec(ctx) // 缺失 key 返回 redis.Nil，忽略

	out := make([]SwitchRatePoint, 0, minutes)
	for i := 0; i < minutes; i++ {
		m := nowMin - int64(minutes-1-i)
		sets := parseCounter(setCmds[i])
		sw := parseCounter(swCmds[i])
		pt := SwitchRatePoint{
			Minute:   time.Unix(m*60, 0),
			Sets:     sets,
			Switches: sw,
		}
		if sets > 0 {
			pt.SwitchRate = float64(sw) / float64(sets)
		}
		out = append(out, pt)
	}
	return out, nil
}

func parseCounter(cmd interface{ Result() (string, error) }) int64 {
	v, err := cmd.Result()
	if err != nil || v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
