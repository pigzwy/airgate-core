package ops

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Service 运维（Ops）域用例编排。
//
// 职责：
//   - 接收插件上报的请求级指标并落库（Report）
//   - 管理员查询：错误日志列表、错误详情、实时大盘概览
//   - 供后台聚合器调用：聚合窗口、清理过期数据
//
// 不碰 gin/http；入参用本包类型，出参用领域类型 + 哨兵错误。
type Service struct {
	repo     Repository
	logger   *slog.Logger
	jobs     *JobRegistry       // 后台任务注册表，注入后系统资源卡可展示 Jobs
	rdb      *redis.Client      // 系统资源卡采集 Redis 连接池用（可选）
	dbStats  func() sql.DBStats // DB 连接池采集（可选）；由装配期注入，避免本层 import ent
	counter  ConcurrencyCounter // 当前并发计数来源（scheduler），可选
	logCtl   LogController      // 运行时日志级别控制（LogSink），可选
	notifier Notifier           // 告警邮件通知器，可选
	config   ConfigProvider     // 运维可调配置来源（settings group=ops），可选

	lastReportDate string // 邮件日报去重：已发送的日期（YYYY-MM-DD）
}

// NewService 构造 Ops Service。
func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

// SetRedis 注入 Redis client（系统资源采集用）。装配期调用一次。
func (s *Service) SetRedis(rdb *redis.Client) {
	s.rdb = rdb
}

// SetDBStatsFunc 注入 DB 连接池统计函数（系统资源采集用）。装配期调用一次。
//
// 用函数注入而非直传 *sql.DB / *ent.Client：本层（service）不得 import ent，
// 而 database/sql 是标准库，sql.DBStats 作为出参不破坏分层。
func (s *Service) SetDBStatsFunc(fn func() sql.DBStats) {
	s.dbStats = fn
}

// Report 落一条插件上报的请求级指标。
//
// 设计：失败只记日志、不返回错误给上报方阻断转发——运维数据丢一条远比拖垮
// 业务请求重要。host 层调用本方法时即使返回 error 也应吞掉。
func (s *Service) Report(ctx context.Context, in ReportInput) error {
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}
	if err := s.repo.Insert(ctx, in); err != nil {
		s.logger.Warn("ops_report_insert_failed",
			"plugin_id", in.PluginID,
			"request_id", in.RequestID,
			"error", err,
		)
		return err
	}
	return nil
}

// ListRequestLogs 分页查询请求/错误日志（管理员）。
func (s *Service) ListRequestLogs(ctx context.Context, f RequestLogFilter) (RequestLogResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	if f.Start != nil && f.End != nil && f.End.Before(*f.Start) {
		return RequestLogResult{}, ErrInvalidTimeRange
	}
	return s.repo.ListRequestLogs(ctx, f)
}

// GetRequestLog 查单条请求日志（错误详情钻取）。
func (s *Service) GetRequestLog(ctx context.Context, id int) (RequestLog, error) {
	return s.repo.GetRequestLog(ctx, id)
}

// Trace 返回一次请求的重试链路（同 client_request_id 的所有日志，按时间升序）。
func (s *Service) Trace(ctx context.Context, clientRequestID string) ([]RequestLog, error) {
	if clientRequestID == "" {
		return nil, nil
	}
	return s.repo.ListTrace(ctx, clientRequestID)
}

// Analytics 按时间范围实时分析（全分位/TTFT/直方图/错误分布/Token/平台维度）。
//
// rangeSeconds<=0 时默认 1 小时；上限 7 天（防止扫描过多原始日志）。
func (s *Service) Analytics(ctx context.Context, rangeSeconds int64, platform string) (Analytics, error) {
	if rangeSeconds <= 0 {
		rangeSeconds = 3600
	}
	const maxRange = int64(7 * 24 * 3600)
	if rangeSeconds > maxRange {
		rangeSeconds = maxRange
	}
	now := time.Now()
	f := AnalyticsFilter{
		Start:    now.Add(-time.Duration(rangeSeconds) * time.Second),
		End:      now,
		Platform: platform,
	}
	return s.repo.QueryAnalytics(ctx, f)
}

// Overview 实时大盘概览：最近一个窗口汇总 + 最近 trendWindows 个窗口趋势。
// platform 为空时取全平台汇总，否则取该平台的趋势。
func (s *Service) Overview(ctx context.Context, trendWindows int, platform string) (Overview, error) {
	if trendWindows <= 0 || trendWindows > 1440 {
		trendWindows = 60 // 默认最近 60 个窗口（1min 窗口即最近 1 小时）；上限 24h
	}
	since := time.Now().Add(-time.Duration(trendWindows) * time.Minute)
	windows, err := s.repo.ListWindowStats(ctx, since, platform)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Trend: windows}
	if len(windows) > 0 {
		out.Latest = windows[len(windows)-1]
	}
	return out, nil
}

// aggregateWindowSeconds 聚合窗口长度（秒）。与聚合器循环周期一致。
const aggregateWindowSeconds = 60

// AggregateOnce 聚合 [windowStart, windowStart+window) 区间的原始日志，写入聚合表。
// 由后台聚合器每个周期调用一次（windowStart 取上一个对齐的整分钟）。
func (s *Service) AggregateOnce(ctx context.Context, windowStart time.Time) error {
	windowStart = windowStart.Truncate(time.Minute)
	end := windowStart.Add(aggregateWindowSeconds * time.Second)

	raws, err := s.repo.AggregateWindow(ctx, windowStart, end)
	if err != nil {
		return err
	}

	stats := make([]WindowStat, 0, len(raws))
	for _, r := range raws {
		ws := WindowStat{
			WindowStart:           windowStart,
			WindowSeconds:         aggregateWindowSeconds,
			Platform:              r.Platform,
			TotalRequests:         r.TotalRequests,
			SuccessRequests:       r.SuccessRequests,
			ErrorRequests:         r.ErrorRequests,
			UpstreamErrorRequests: r.UpstreamErrorRequests,
			P50DurationMs:         r.P50DurationMs,
			P95DurationMs:         r.P95DurationMs,
			P99DurationMs:         r.P99DurationMs,
			TotalInputTokens:      r.TotalInputTokens,
			TotalOutputTokens:     r.TotalOutputTokens,
		}
		if aggregateWindowSeconds > 0 {
			ws.RPS = float64(r.TotalRequests) / float64(aggregateWindowSeconds)
		}
		if r.TotalRequests > 0 {
			ws.ErrorRate = float64(r.ErrorRequests) / float64(r.TotalRequests)
		}
		stats = append(stats, ws)
	}

	if len(stats) == 0 {
		return nil // 窗口内无请求，不写空行
	}
	return s.repo.UpsertWindowStats(ctx, windowStart, aggregateWindowSeconds, stats)
}

// Purge 清理保留期之外的请求日志与聚合窗口。由后台清理任务调用。
//
// logRetention 控制原始日志保留时长；statRetention 控制聚合窗口保留时长
// （聚合窗口体积小，可比原始日志保留更久）。
func (s *Service) Purge(ctx context.Context, logRetention, statRetention, sysLogRetention time.Duration) error {
	now := time.Now()
	if logRetention > 0 {
		n, err := s.repo.PurgeRequestLogsBefore(ctx, now.Add(-logRetention))
		if err != nil {
			return err
		}
		if n > 0 {
			s.logger.Info("ops_request_log_purged", "count", n, "retention", logRetention.String())
		}
	}
	if statRetention > 0 {
		n, err := s.repo.PurgeWindowStatsBefore(ctx, now.Add(-statRetention))
		if err != nil {
			return err
		}
		if n > 0 {
			s.logger.Info("ops_window_stat_purged", "count", n, "retention", statRetention.String())
		}
	}
	// 系统日志按独立保留期清理。
	if sysLogRetention > 0 {
		n, err := s.repo.PurgeSystemLogsBefore(ctx, now.Add(-sysLogRetention))
		if err != nil {
			return err
		}
		if n > 0 {
			s.logger.Info("ops_system_log_purged", "count", n, "retention", sysLogRetention.String())
		}
	}
	return nil
}

// MVP 默认保留期（后续可配置化）。
const (
	defaultLogRetention    = 7 * 24 * time.Hour  // 原始请求日志保留 7 天
	defaultStatRetention   = 30 * 24 * time.Hour // 聚合窗口保留 30 天（体积小，留久点）
	defaultSysLogRetention = 7 * 24 * time.Hour  // 系统日志保留 7 天
	purgeInterval          = time.Hour           // 清理周期
)

// StartAggregationLoop 启动后台聚合 + 清理循环。由 server 在启动期调用，ctx 取消即退出。
//
// 聚合：每分钟对齐时跑一次，聚合「上一个完整整分钟窗口」（避免聚合还在写入的当前窗口）。
// 清理：每小时跑一次，按保留期删过期数据。
func (s *Service) StartAggregationLoop(ctx context.Context) {
	go s.runAggregationLoop(ctx)
	go s.runPurgeLoop(ctx)
	go s.runAlertLoop(ctx)
}

func (s *Service) runAggregationLoop(ctx context.Context) {
	var job *JobHandle
	if s.jobs != nil {
		job = s.jobs.Register("ops_aggregation")
	}
	ticker := time.NewTicker(aggregateWindowSeconds * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if job != nil {
				job.Heartbeat()
			}
			// 聚合上一个完整整分钟窗口
			prevWindow := time.Now().Add(-time.Minute).Truncate(time.Minute)
			if err := s.AggregateOnce(ctx, prevWindow); err != nil {
				s.logger.Warn("ops_aggregate_window_failed",
					"window_start", prevWindow.Format(time.RFC3339), "error", err)
				if job != nil {
					job.MarkError(err)
				}
			} else if job != nil {
				job.MarkRun()
			}
		}
	}
}

func (s *Service) runPurgeLoop(ctx context.Context) {
	var job *JobHandle
	if s.jobs != nil {
		job = s.jobs.Register("ops_purge")
	}
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if job != nil {
				job.Heartbeat()
			}
			logRet, statRet, sysRet := s.retentionDurations(ctx)
			if err := s.Purge(ctx, logRet, statRet, sysRet); err != nil {
				s.logger.Warn("ops_purge_failed", "error", err)
				if job != nil {
					job.MarkError(err)
				}
			} else if job != nil {
				job.MarkRun()
			}
		}
	}
}
