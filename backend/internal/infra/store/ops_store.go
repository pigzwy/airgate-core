package store

import (
	"context"
	"sort"
	"time"

	"github.com/DouDOU-start/airgate-core/ent"
	entopslog "github.com/DouDOU-start/airgate-core/ent/opsrequestlog"
	entopswin "github.com/DouDOU-start/airgate-core/ent/opswindowstat"
	appops "github.com/DouDOU-start/airgate-core/internal/app/ops"
)

// OpsStore 使用 Ent 实现运维（Ops）域仓储。
// 仅本层 import ent；上层 service 通过 appops.Repository 接口访问。
type OpsStore struct {
	db *ent.Client
}

// NewOpsStore 创建 Ops 仓储。
func NewOpsStore(db *ent.Client) *OpsStore {
	return &OpsStore{db: db}
}

// 编译期接口断言。
var _ appops.Repository = (*OpsStore)(nil)

// Insert 落一条请求级日志。
func (s *OpsStore) Insert(ctx context.Context, in appops.ReportInput) error {
	createdAt := in.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := s.db.OpsRequestLog.Create().
		SetRequestID(in.RequestID).
		SetPluginID(in.PluginID).
		SetPlatform(in.Platform).
		SetModel(in.Model).
		SetEndpoint(in.Endpoint).
		SetUserIDSnapshot(in.UserID).
		SetAPIKeyIDSnapshot(in.APIKeyID).
		SetAccountIDSnapshot(in.AccountID).
		SetGroupIDSnapshot(in.GroupID).
		SetSuccess(in.Success).
		SetStatusCode(in.StatusCode).
		SetUpstreamStatusCode(in.UpstreamStatusCode).
		SetDurationMs(in.DurationMs).
		SetFirstTokenMs(in.FirstTokenMs).
		SetStream(in.Stream).
		SetInputTokens(in.InputTokens).
		SetOutputTokens(in.OutputTokens).
		SetErrorKind(in.ErrorKind).
		SetErrorMsg(in.ErrorMsg).
		SetErrorDetail(in.ErrorDetail).
		SetCreatedAt(createdAt).
		Save(ctx)
	return err
}

// ListRequestLogs 分页查询请求/错误日志。
func (s *OpsStore) ListRequestLogs(ctx context.Context, f appops.RequestLogFilter) (appops.RequestLogResult, error) {
	q := s.db.OpsRequestLog.Query()
	if f.OnlyErrors {
		q = q.Where(entopslog.SuccessEQ(false))
	}
	if f.Platform != "" {
		q = q.Where(entopslog.PlatformEQ(f.Platform))
	}
	if f.Model != "" {
		q = q.Where(entopslog.ModelEQ(f.Model))
	}
	if f.ErrorKind != "" {
		q = q.Where(entopslog.ErrorKindEQ(f.ErrorKind))
	}
	if f.Start != nil {
		q = q.Where(entopslog.CreatedAtGTE(*f.Start))
	}
	if f.End != nil {
		q = q.Where(entopslog.CreatedAtLT(*f.End))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return appops.RequestLogResult{}, err
	}

	rows, err := q.
		Order(ent.Desc(entopslog.FieldCreatedAt)).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		All(ctx)
	if err != nil {
		return appops.RequestLogResult{}, err
	}

	list := make([]appops.RequestLog, 0, len(rows))
	for _, r := range rows {
		list = append(list, toRequestLog(r))
	}
	return appops.RequestLogResult{
		List:     list,
		Total:    int64(total),
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

// GetRequestLog 按 ID 查单条。
func (s *OpsStore) GetRequestLog(ctx context.Context, id int) (appops.RequestLog, error) {
	r, err := s.db.OpsRequestLog.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return appops.RequestLog{}, appops.ErrRequestLogNotFound
		}
		return appops.RequestLog{}, err
	}
	return toRequestLog(r), nil
}

// aggMaxSampleRows 单个窗口聚合时拉取的最大行数。1 分钟窗口正常远低于此值；
// 超大流量下截断为采样，分位数为近似值（MVP 可接受，避免一次拉太多行进内存）。
const aggMaxSampleRows = 20000

// aggSampleRow 聚合扫描用的精简行（只取计算需要的列）。
type aggSampleRow struct {
	Platform           string `json:"platform"`
	Success            bool   `json:"success"`
	UpstreamStatusCode int    `json:"upstream_status_code"`
	DurationMs         int64  `json:"duration_ms"`
	InputTokens        int    `json:"input_tokens"`
	OutputTokens       int    `json:"output_tokens"`
}

// AggregateWindow 聚合 [start, end) 区间，按 platform 分组 + 一条全平台汇总（platform=""）。
//
// 实现：拉取窗口内精简行到内存，在 Go 侧分组计算 count/分位数/sum。
// 选择 Go 侧聚合而非 SQL percentile_cont，是为了不引入 ent 的 sql/modifier feature
// （会改全局生成配置、影响 verify-ent CI），保持改动最小、零外溢。1 分钟窗口行数可控，
// 超过 aggMaxSampleRows 时截断为采样（分位数近似）。
func (s *OpsStore) AggregateWindow(ctx context.Context, start, end time.Time) ([]appops.RawAggregate, error) {
	var rows []aggSampleRow
	err := s.db.OpsRequestLog.Query().
		Where(
			entopslog.CreatedAtGTE(start),
			entopslog.CreatedAtLT(end),
		).
		Limit(aggMaxSampleRows).
		Select(
			entopslog.FieldPlatform,
			entopslog.FieldSuccess,
			entopslog.FieldUpstreamStatusCode,
			entopslog.FieldDurationMs,
			entopslog.FieldInputTokens,
			entopslog.FieldOutputTokens,
		).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// 按 platform 分桶，外加一个全平台桶（key ""）。
	buckets := map[string]*aggBucket{"": newAggBucket("")}
	for _, r := range rows {
		buckets[""].add(r)
		b, ok := buckets[r.Platform]
		if !ok {
			b = newAggBucket(r.Platform)
			buckets[r.Platform] = b
		}
		b.add(r)
	}

	out := make([]appops.RawAggregate, 0, len(buckets))
	// 全平台汇总放第一条
	out = append(out, buckets[""].result())
	for k, b := range buckets {
		if k == "" {
			continue
		}
		out = append(out, b.result())
	}
	return out, nil
}

// aggBucket 单个分组（platform）的聚合累加器。
type aggBucket struct {
	platform     string
	total        int64
	success      int64
	errs         int64
	upstreamErr  int64
	inputTokens  int64
	outputTokens int64
	durations    []int64
}

func newAggBucket(platform string) *aggBucket {
	return &aggBucket{platform: platform}
}

func (b *aggBucket) add(r aggSampleRow) {
	b.total++
	if r.Success {
		b.success++
	} else {
		b.errs++
	}
	if r.UpstreamStatusCode >= 500 {
		b.upstreamErr++
	}
	b.inputTokens += int64(r.InputTokens)
	b.outputTokens += int64(r.OutputTokens)
	b.durations = append(b.durations, r.DurationMs)
}

func (b *aggBucket) result() appops.RawAggregate {
	sort.Slice(b.durations, func(i, j int) bool { return b.durations[i] < b.durations[j] })
	return appops.RawAggregate{
		Platform:              b.platform,
		TotalRequests:         b.total,
		SuccessRequests:       b.success,
		ErrorRequests:         b.errs,
		UpstreamErrorRequests: b.upstreamErr,
		P50DurationMs:         percentile(b.durations, 0.50),
		P95DurationMs:         percentile(b.durations, 0.95),
		P99DurationMs:         percentile(b.durations, 0.99),
		TotalInputTokens:      b.inputTokens,
		TotalOutputTokens:     b.outputTokens,
	}
}

// percentile 返回已排序切片的 p 分位（nearest-rank）。空切片返回 0。
func percentile(sorted []int64, p float64) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := int(p * float64(n-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// UpsertWindowStats 写入一个窗口的聚合结果。
//
// 用「先删该窗口所有行，再批量插入」替代 upsert，避免引入 ent 的 sql/upsert feature。
// 聚合器对同一窗口可能重算（如补算），delete+insert 保证幂等。
func (s *OpsStore) UpsertWindowStats(ctx context.Context, windowStart time.Time, windowSeconds int, stats []appops.WindowStat) error {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	// 删除该窗口已有聚合行
	if _, err := tx.OpsWindowStat.Delete().
		Where(entopswin.WindowStartEQ(windowStart)).
		Exec(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	// 批量插入
	builders := make([]*ent.OpsWindowStatCreate, 0, len(stats))
	for _, st := range stats {
		builders = append(builders, tx.OpsWindowStat.Create().
			SetWindowStart(windowStart).
			SetWindowSeconds(windowSeconds).
			SetPlatform(st.Platform).
			SetTotalRequests(st.TotalRequests).
			SetSuccessRequests(st.SuccessRequests).
			SetErrorRequests(st.ErrorRequests).
			SetUpstreamErrorRequests(st.UpstreamErrorRequests).
			SetRps(st.RPS).
			SetErrorRate(st.ErrorRate).
			SetP50DurationMs(st.P50DurationMs).
			SetP95DurationMs(st.P95DurationMs).
			SetP99DurationMs(st.P99DurationMs).
			SetTotalInputTokens(st.TotalInputTokens).
			SetTotalOutputTokens(st.TotalOutputTokens))
	}
	if _, err := tx.OpsWindowStat.CreateBulk(builders...).Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ListWindowStats 查全平台汇总（platform=""）最近 since 之后的窗口，按时间升序。
func (s *OpsStore) ListWindowStats(ctx context.Context, since time.Time) ([]appops.WindowStat, error) {
	rows, err := s.db.OpsWindowStat.Query().
		Where(
			entopswin.PlatformEQ(""),
			entopswin.WindowStartGTE(since),
		).
		Order(ent.Asc(entopswin.FieldWindowStart)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.WindowStat, 0, len(rows))
	for _, r := range rows {
		out = append(out, toWindowStat(r))
	}
	return out, nil
}

// PurgeRequestLogsBefore 删除 before 之前的请求日志。
func (s *OpsStore) PurgeRequestLogsBefore(ctx context.Context, before time.Time) (int, error) {
	return s.db.OpsRequestLog.Delete().
		Where(entopslog.CreatedAtLT(before)).
		Exec(ctx)
}

// PurgeWindowStatsBefore 删除 before 之前的聚合窗口。
func (s *OpsStore) PurgeWindowStatsBefore(ctx context.Context, before time.Time) (int, error) {
	return s.db.OpsWindowStat.Delete().
		Where(entopswin.WindowStartLT(before)).
		Exec(ctx)
}

// ---- 映射 helper ----

func toRequestLog(r *ent.OpsRequestLog) appops.RequestLog {
	return appops.RequestLog{
		ID:                 r.ID,
		RequestID:          r.RequestID,
		PluginID:           r.PluginID,
		Platform:           r.Platform,
		Model:              r.Model,
		Endpoint:           r.Endpoint,
		UserID:             r.UserIDSnapshot,
		APIKeyID:           r.APIKeyIDSnapshot,
		AccountID:          r.AccountIDSnapshot,
		GroupID:            r.GroupIDSnapshot,
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
		CreatedAt:          r.CreatedAt,
	}
}

func toWindowStat(r *ent.OpsWindowStat) appops.WindowStat {
	return appops.WindowStat{
		WindowStart:           r.WindowStart,
		WindowSeconds:         r.WindowSeconds,
		Platform:              r.Platform,
		TotalRequests:         r.TotalRequests,
		SuccessRequests:       r.SuccessRequests,
		ErrorRequests:         r.ErrorRequests,
		UpstreamErrorRequests: r.UpstreamErrorRequests,
		RPS:                   r.Rps,
		ErrorRate:             r.ErrorRate,
		P50DurationMs:         r.P50DurationMs,
		P95DurationMs:         r.P95DurationMs,
		P99DurationMs:         r.P99DurationMs,
		TotalInputTokens:      r.TotalInputTokens,
		TotalOutputTokens:     r.TotalOutputTokens,
	}
}
