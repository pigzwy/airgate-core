package ops

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

// ===== 系统日志（M11）：slog sink 异步落库 + 运行时级别控制 =====

// SystemLogEntry 系统日志领域对象。
type SystemLogEntry struct {
	ID        int
	Level     string
	Component string
	Message   string
	RequestID string
	Attrs     string // 结构化字段 JSON
	CreatedAt time.Time
}

// SystemLogFilter 系统日志查询条件。
type SystemLogFilter struct {
	Page      int
	PageSize  int
	Level     string // 精确匹配；空=不限
	Component string
	RequestID string
	Keyword   string // message 模糊匹配
	Start     *time.Time
	End       *time.Time
}

// SystemLogResult 系统日志分页结果。
type SystemLogResult struct {
	List     []SystemLogEntry
	Total    int64
	Page     int
	PageSize int
}

// LogController 运行时日志级别控制（由 LogSink 实现，注入 Service 供 API 调用）。
type LogController interface {
	Level() string
	SetLevel(level string) error
	ResetLevel()
	DefaultLevel() string
	Dropped() int64
}

// ParseLevel 把字符串日志级别转为 slog.Level。无法识别返回 info。
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// logSinkBuffer 异步队列容量。满则丢弃（背压保护），由 Dropped() 暴露丢弃计数。
const logSinkBuffer = 4096

// sinkRecord 进入异步队列的一条日志。
type sinkRecord struct {
	level     string
	component string
	message   string
	requestID string
	attrs     string
	createdAt time.Time
}

// LogSink 是一个包装型 slog.Handler：
//   - 把每条日志透传给 inner（保持原有 stdout 输出）
//   - 同时异步入队、批量落库（OpsSystemLog），供后台日志查询
//   - 持有共享的 *slog.LevelVar，支持运行时调级（同时影响 inner 的 Enabled）
//
// 入队非阻塞，队列满即丢弃，绝不拖慢业务日志。worker 自身错误不再走 slog（防递归）。
type LogSink struct {
	inner        slog.Handler
	level        *slog.LevelVar
	defaultLevel slog.Level
	repo         Repository
	ch           chan sinkRecord
	preset       []slog.Attr
	dropped      *atomic.Int64
}

// NewLogSink 创建日志 sink。inner 为底层输出 handler（须与 level 共享 LevelVar）。
func NewLogSink(repo Repository, inner slog.Handler, level *slog.LevelVar) *LogSink {
	return &LogSink{
		inner:        inner,
		level:        level,
		defaultLevel: level.Level(),
		repo:         repo,
		ch:           make(chan sinkRecord, logSinkBuffer),
		dropped:      &atomic.Int64{},
	}
}

// Enabled 由共享 LevelVar 控制，运行时调级即时生效。
func (h *LogSink) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

// Handle 透传 inner 并异步入队落库。
func (h *LogSink) Handle(ctx context.Context, r slog.Record) error {
	attrs := make(map[string]any, r.NumAttrs()+len(h.preset))
	for _, a := range h.preset {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	rec := sinkRecord{
		level:     r.Level.String(),
		component: asString(attrs["component"]),
		message:   r.Message,
		requestID: firstString(attrs, "request_id", "client_request_id", "requestID"),
		createdAt: r.Time,
	}
	if rec.createdAt.IsZero() {
		rec.createdAt = time.Now()
	}
	if len(attrs) > 0 {
		if b, err := json.Marshal(attrs); err == nil {
			rec.attrs = string(b)
		}
	}

	select {
	case h.ch <- rec:
	default:
		h.dropped.Add(1)
	}

	return h.inner.Handle(ctx, r)
}

// WithAttrs 累积预设字段（同步透传 inner）。
func (h *LogSink) WithAttrs(as []slog.Attr) slog.Handler {
	n := *h
	n.inner = h.inner.WithAttrs(as)
	n.preset = append(append([]slog.Attr{}, h.preset...), as...)
	return &n
}

// WithGroup 透传 inner（DB 记录不分组，保持扁平）。
func (h *LogSink) WithGroup(name string) slog.Handler {
	n := *h
	n.inner = h.inner.WithGroup(name)
	return &n
}

// Start 启动后台落库 worker。ctx 取消即退出（退出前尽量冲刷队列）。
func (h *LogSink) Start(ctx context.Context) {
	go h.run(ctx)
}

func (h *LogSink) run(ctx context.Context) {
	const batchMax = 200
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	batch := make([]SystemLogEntry, 0, batchMax)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// worker 自身错误不走 slog（防递归），落库失败丢弃本批。
		_ = h.repo.InsertSystemLogs(context.WithoutCancel(ctx), batch)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// 冲刷剩余队列
			for {
				select {
				case rec := <-h.ch:
					batch = append(batch, toSystemLogEntry(rec))
					if len(batch) >= batchMax {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case rec := <-h.ch:
			batch = append(batch, toSystemLogEntry(rec))
			if len(batch) >= batchMax {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func toSystemLogEntry(r sinkRecord) SystemLogEntry {
	return SystemLogEntry{
		Level:     strings.ToLower(r.level),
		Component: r.component,
		Message:   r.message,
		RequestID: r.requestID,
		Attrs:     r.attrs,
		CreatedAt: r.createdAt,
	}
}

// ---- LogController 实现 ----

func (h *LogSink) Level() string { return strings.ToLower(h.level.Level().String()) }

func (h *LogSink) SetLevel(level string) error {
	h.level.Set(ParseLevel(level))
	return nil
}

func (h *LogSink) ResetLevel() { h.level.Set(h.defaultLevel) }

func (h *LogSink) DefaultLevel() string { return strings.ToLower(h.defaultLevel.String()) }

func (h *LogSink) Dropped() int64 { return h.dropped.Load() }

// ---- service 层方法 ----

// SetLogController 注入日志级别控制器（LogSink）。装配期调用一次。
func (s *Service) SetLogController(c LogController) {
	s.logCtl = c
}

// ListSystemLogs 分页查询系统日志。
func (s *Service) ListSystemLogs(ctx context.Context, f SystemLogFilter) (SystemLogResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	if f.Start != nil && f.End != nil && f.End.Before(*f.Start) {
		return SystemLogResult{}, ErrInvalidTimeRange
	}
	return s.repo.ListSystemLogs(ctx, f)
}

// LogLevel 返回当前与默认日志级别。logCtl 未注入时返回空。
func (s *Service) LogLevel() (current, def string, dropped int64) {
	if s.logCtl == nil {
		return "", "", 0
	}
	return s.logCtl.Level(), s.logCtl.DefaultLevel(), s.logCtl.Dropped()
}

// SetLogLevel 运行时调整日志级别。logCtl 未注入时静默忽略。
func (s *Service) SetLogLevel(level string) error {
	if s.logCtl == nil {
		return nil
	}
	return s.logCtl.SetLevel(level)
}

// ResetLogLevel 恢复到启动时的默认级别。
func (s *Service) ResetLogLevel() {
	if s.logCtl != nil {
		s.logCtl.ResetLevel()
	}
}

// ---- helpers ----

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := asString(m[k]); s != "" {
			return s
		}
	}
	return ""
}
