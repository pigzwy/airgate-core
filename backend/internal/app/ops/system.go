package ops

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SystemMetrics Core 进程的系统资源指标快照（M1 第一版：零外部依赖）。
//
// 采集来源（全部 Core 自有，不依赖插件）：
//   - Goroutines：runtime.NumGoroutine()
//   - 内存：runtime.MemStats（HeapAlloc 已用堆 / Sys 向 OS 申请总量）
//   - Redis：go-redis PoolStats（连接池命中/未命中/总数/空闲）
//   - Jobs：Core 后台循环注册表（见 JobRegistry）
//
// CPU 使用率 / DB 连接池在 M1 增强（需 gopsutil + ent driver 断言），暂不含。
type SystemMetrics struct {
	Goroutines int           `json:"goroutines"`
	Memory     MemoryMetrics `json:"memory"`
	DB         DBMetrics     `json:"db"`
	Redis      RedisMetrics  `json:"redis"`
	Jobs       []JobMetrics  `json:"jobs"`
	CapturedAt time.Time     `json:"captured_at"`
}

// DBMetrics 数据库连接池指标（database/sql DBStats 映射）。
type DBMetrics struct {
	MaxOpenConns      int   `json:"max_open_conns"`      // 最大连接数上限（0=不限）
	OpenConns         int   `json:"open_conns"`          // 当前打开连接数（使用中+空闲）
	InUse             int   `json:"in_use"`              // 使用中连接数
	Idle              int   `json:"idle"`                // 空闲连接数
	WaitCount         int64 `json:"wait_count"`          // 累计等待连接次数
	WaitDurationMs    int64 `json:"wait_duration_ms"`    // 累计等待连接耗时（毫秒）
	MaxIdleClosed     int64 `json:"max_idle_closed"`     // 因 MaxIdleConns 被关闭数
	MaxLifetimeClosed int64 `json:"max_lifetime_closed"` // 因 ConnMaxLifetime 被关闭数
}

// MemoryMetrics 内存指标。
type MemoryMetrics struct {
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"` // 已分配且未回收的堆字节
	SysBytes       uint64 `json:"sys_bytes"`        // 向 OS 申请的字节总量
	NumGC          uint32 `json:"num_gc"`           // GC 次数
}

// RedisMetrics Redis 连接池指标（go-redis PoolStats 映射）。
type RedisMetrics struct {
	Hits       uint32 `json:"hits"`        // 复用连接命中数
	Misses     uint32 `json:"misses"`      // 未命中（需新建连接）数
	Timeouts   uint32 `json:"timeouts"`    // 等待连接超时数
	TotalConns uint32 `json:"total_conns"` // 池中连接总数
	IdleConns  uint32 `json:"idle_conns"`  // 空闲连接数
	StaleConns uint32 `json:"stale_conns"` // 过期连接数
}

// JobMetrics 单个后台任务的运行状态。
type JobMetrics struct {
	Name          string     `json:"name"`           // 任务名（如 ops_aggregation / ops_purge / billing_recorder）
	Running       bool       `json:"running"`        // 是否已注册（在跑）
	LastHeartbeat *time.Time `json:"last_heartbeat"` // 最近一次心跳
	LastRunAt     *time.Time `json:"last_run_at"`    // 最近一次完成一轮（由任务自行标记）
	LastError     string     `json:"last_error"`     // 最近一次错误（空=无）
}

// JobRegistry 后台任务注册表。
//
// 各后台循环启动时调 Register(name) 拿到一个 *JobHandle，
// 每轮执行时调 Heartbeat() / MarkRun() / MarkError(err) 上报状态。
// SystemMetrics() 读取注册表生成 Jobs 卡片数据。
//
// 设计：读多写少，用 RWMutex 即可；不引入额外依赖。
type JobRegistry struct {
	mu    sync.RWMutex
	jobs  map[string]*JobHandle
	clock func() time.Time // 可注入，测试用；生产用 time.Now
}

// NewJobRegistry 创建后台任务注册表。
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{
		jobs:  make(map[string]*JobHandle),
		clock: time.Now,
	}
}

// JobHandle 单个后台任务的句柄，循环里持有它上报状态。
type JobHandle struct {
	registry      *JobRegistry
	name          string
	mu            sync.Mutex
	lastHeartbeat time.Time
	lastRunAt     time.Time
	lastError     string
}

// Register 注册一个后台任务，返回句柄。重复注册同名任务会被覆盖（重启场景）。
func (r *JobRegistry) Register(name string) *JobHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := &JobHandle{registry: r, name: name}
	r.jobs[name] = h
	return h
}

// Heartbeat 上报「我还活着」。循环每轮执行前调一次。
func (h *JobHandle) Heartbeat() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastHeartbeat = h.registry.clock()
}

// MarkRun 标记完成一轮。
func (h *JobHandle) MarkRun() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastRunAt = h.registry.clock()
	h.lastError = ""
}

// MarkError 标记本轮出错。
func (h *JobHandle) MarkError(err error) {
	if err == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastError = err.Error()
	h.lastRunAt = h.registry.clock()
}

// snapshot 读所有任务状态。
func (r *JobRegistry) snapshot() []JobMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]JobMetrics, 0, len(r.jobs))
	for name, h := range r.jobs {
		h.mu.Lock()
		var hb, run *time.Time
		if !h.lastHeartbeat.IsZero() {
			t := h.lastHeartbeat
			hb = &t
		}
		if !h.lastRunAt.IsZero() {
			t := h.lastRunAt
			run = &t
		}
		out = append(out, JobMetrics{
			Name:          name,
			Running:       true,
			LastHeartbeat: hb,
			LastRunAt:     run,
			LastError:     h.lastError,
		})
		h.mu.Unlock()
	}
	return out
}

// SystemMetrics 采集一次系统资源快照（管理员大盘用）。
//
// Redis client 由 SetRedis 注入；未注入时 Redis 指标返回零值（降级，不崩）。
func (s *Service) SystemMetrics(ctx context.Context) SystemMetrics {
	return s.collectSystemMetrics(ctx, s.rdb)
}

// collectSystemMetrics 采集实现。rdb 为 nil 时 Redis 指标返回零值。
func (s *Service) collectSystemMetrics(ctx context.Context, rdb *redis.Client) SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	out := SystemMetrics{
		Goroutines: runtime.NumGoroutine(),
		Memory: MemoryMetrics{
			HeapAllocBytes: m.HeapAlloc,
			SysBytes:       m.Sys,
			NumGC:          m.NumGC,
		},
		CapturedAt: time.Now(),
	}

	if s.dbStats != nil {
		ds := s.dbStats()
		out.DB = DBMetrics{
			MaxOpenConns:      ds.MaxOpenConnections,
			OpenConns:         ds.OpenConnections,
			InUse:             ds.InUse,
			Idle:              ds.Idle,
			WaitCount:         ds.WaitCount,
			WaitDurationMs:    ds.WaitDuration.Milliseconds(),
			MaxIdleClosed:     ds.MaxIdleClosed,
			MaxLifetimeClosed: ds.MaxLifetimeClosed,
		}
	}

	if rdb != nil {
		ps := rdb.PoolStats()
		out.Redis = RedisMetrics{
			Hits:       ps.Hits,
			Misses:     ps.Misses,
			Timeouts:   ps.Timeouts,
			TotalConns: ps.TotalConns,
			IdleConns:  ps.IdleConns,
			StaleConns: ps.StaleConns,
		}
	}

	if s.jobs != nil {
		out.Jobs = s.jobs.snapshot()
	}
	_ = ctx
	return out
}

// SetJobRegistry 注入后台任务注册表。由 server 在装配期调用一次。
func (s *Service) SetJobRegistry(r *JobRegistry) {
	s.jobs = r
}
