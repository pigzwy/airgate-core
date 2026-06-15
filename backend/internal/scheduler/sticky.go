package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// stickyTTL 粘性会话默认过期时间
	stickyTTL = 30 * time.Minute
	// switchCounterTTL 切换率统计计数器保留时长（覆盖 24h 趋势查询）。
	switchCounterTTL = 25 * time.Hour
)

// StickySetKey 返回某一分钟桶内「粘性绑定次数」计数器 Redis Key。
// minute 为 unix 秒 / 60。运维侧（app/ops）按此格式读取以算切换率。
func StickySetKey(minute int64) string {
	return fmt.Sprintf("ops:sticky:set:%d", minute)
}

// StickySwitchKey 返回某一分钟桶内「账号切换次数」计数器 Redis Key。
func StickySwitchKey(minute int64) string {
	return fmt.Sprintf("ops:sticky:switch:%d", minute)
}

// StickySession 粘性会话管理
// 通过 Redis 缓存 session → account 映射，实现对话上下文连续性
type StickySession struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewStickySession 创建粘性会话管理器
func NewStickySession(rdb *redis.Client) *StickySession {
	return &StickySession{
		rdb: rdb,
		ttl: stickyTTL,
	}
}

// stickyKey 生成 Redis Key
// 格式：sticky:{user_id}:{platform}:{session_id}
func stickyKey(userID int, platform, sessionID string) string {
	return fmt.Sprintf("sticky:%d:%s:%s", userID, platform, sessionID)
}

// Get 获取粘性会话绑定的账户 ID
func (s *StickySession) Get(ctx context.Context, userID int, platform, sessionID string) (accountID int, found bool) {
	if s.rdb == nil {
		return 0, false
	}

	key := stickyKey(userID, platform, sessionID)
	val, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return 0, false
	}

	id, err := strconv.Atoi(val)
	if err != nil {
		return 0, false
	}
	return id, true
}

// Set 设置粘性会话绑定（同时续期 TTL）
func (s *StickySession) Set(ctx context.Context, userID int, platform, sessionID string, accountID int) {
	if s.rdb == nil {
		return
	}

	key := stickyKey(userID, platform, sessionID)

	// 切换率统计（M13）：按分钟桶记录绑定次数与「账号变化」次数。
	// 绑定到与上次不同的账号即视为一次切换（粘性会话被打破/重路由）。
	minute := time.Now().Unix() / 60
	old, err := s.rdb.Get(ctx, key).Result()
	setKey := StickySetKey(minute)
	s.rdb.Incr(ctx, setKey)
	s.rdb.Expire(ctx, setKey, switchCounterTTL)
	if err == nil && old != "" && old != strconv.Itoa(accountID) {
		swKey := StickySwitchKey(minute)
		s.rdb.Incr(ctx, swKey)
		s.rdb.Expire(ctx, swKey, switchCounterTTL)
	}

	s.rdb.Set(ctx, key, strconv.Itoa(accountID), s.ttl)
}
