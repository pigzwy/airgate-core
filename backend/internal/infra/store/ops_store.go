package store

import (
	"context"
	"sort"
	"time"

	"github.com/DouDOU-start/airgate-core/ent"
	entopslog "github.com/DouDOU-start/airgate-core/ent/opsrequestlog"
	entopssyslog "github.com/DouDOU-start/airgate-core/ent/opssystemlog"
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
		SetClientRequestID(in.ClientRequestID).
		SetPluginID(in.PluginID).
		SetPlatform(in.Platform).
		SetModel(in.Model).
		SetUpstreamModel(in.UpstreamModel).
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
	// 成功态过滤：SuccessMode 优先于向后兼容的 OnlyErrors。
	switch f.SuccessMode {
	case "success":
		q = q.Where(entopslog.SuccessEQ(true))
	case "error":
		q = q.Where(entopslog.SuccessEQ(false))
	case "all":
		// 不过滤
	default:
		if f.OnlyErrors {
			q = q.Where(entopslog.SuccessEQ(false))
		}
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
	if f.MinDurationMs > 0 {
		q = q.Where(entopslog.DurationMsGTE(f.MinDurationMs))
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

	// 排序：默认按时间降序；支持按耗时排序（慢请求 Top）。
	field := entopslog.FieldCreatedAt
	if f.SortBy == "duration_ms" {
		field = entopslog.FieldDurationMs
	}
	dir := ent.Asc
	if f.SortDesc || f.SortBy == "" {
		dir = ent.Desc
	}

	rows, err := q.
		Order(dir(field)).
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

// ListTrace 返回同一 client_request_id 的所有请求日志（按时间升序），即重试链路。
func (s *OpsStore) ListTrace(ctx context.Context, clientRequestID string) ([]appops.RequestLog, error) {
	rows, err := s.db.OpsRequestLog.Query().
		Where(entopslog.ClientRequestIDEQ(clientRequestID)).
		Order(ent.Asc(entopslog.FieldCreatedAt)).
		Limit(50).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.RequestLog, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRequestLog(r))
	}
	return out, nil
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

// analyticsMaxSampleRows 分析查询拉取的最大行数。超过则截断为采样（分位近似），
// 并在 Summary.Sampled 置位提示前端。时间范围越大越可能触顶。
const analyticsMaxSampleRows = 50000

// analyticsRow 分析扫描用的精简行。
type analyticsRow struct {
	Platform           string `json:"platform"`
	Model              string `json:"model"`
	Success            bool   `json:"success"`
	StatusCode         int    `json:"status_code"`
	UpstreamStatusCode int    `json:"upstream_status_code"`
	DurationMs         int64  `json:"duration_ms"`
	FirstTokenMs       int64  `json:"first_token_ms"`
	Stream             bool   `json:"stream"`
	InputTokens        int    `json:"input_tokens"`
	OutputTokens       int    `json:"output_tokens"`
}

// latencyBuckets 延迟直方图桶边界（毫秒）。最后一桶 Max=0 表示无上限。
var latencyBuckets = []appops.HistogramBucket{
	{Label: "0-50ms", Min: 0, Max: 50},
	{Label: "50-100ms", Min: 50, Max: 100},
	{Label: "100-300ms", Min: 100, Max: 300},
	{Label: "300-500ms", Min: 300, Max: 500},
	{Label: "500ms-1s", Min: 500, Max: 1000},
	{Label: "1-3s", Min: 1000, Max: 3000},
	{Label: "3-10s", Min: 3000, Max: 10000},
	{Label: ">10s", Min: 10000, Max: 0},
}

// QueryAnalytics 按时间范围从原始日志实时计算全套分析指标。
func (s *OpsStore) QueryAnalytics(ctx context.Context, f appops.AnalyticsFilter) (appops.Analytics, error) {
	q := s.db.OpsRequestLog.Query().
		Where(
			entopslog.CreatedAtGTE(f.Start),
			entopslog.CreatedAtLT(f.End),
		)
	if f.Platform != "" {
		q = q.Where(entopslog.PlatformEQ(f.Platform))
	}

	var rows []analyticsRow
	err := q.
		Limit(analyticsMaxSampleRows).
		Select(
			entopslog.FieldPlatform,
			entopslog.FieldModel,
			entopslog.FieldSuccess,
			entopslog.FieldStatusCode,
			entopslog.FieldUpstreamStatusCode,
			entopslog.FieldDurationMs,
			entopslog.FieldFirstTokenMs,
			entopslog.FieldStream,
			entopslog.FieldInputTokens,
			entopslog.FieldOutputTokens,
		).
		Scan(ctx, &rows)
	if err != nil {
		return appops.Analytics{}, err
	}

	rangeSeconds := int64(f.End.Sub(f.Start).Seconds())
	if rangeSeconds <= 0 {
		rangeSeconds = 1
	}

	out := appops.Analytics{
		Summary: appops.AnalyticsSummary{
			RangeSeconds: rangeSeconds,
			Sampled:      len(rows) >= analyticsMaxSampleRows,
		},
		Histogram: make([]appops.HistogramBucket, len(latencyBuckets)),
	}
	copy(out.Histogram, latencyBuckets)

	if len(rows) == 0 {
		return out, nil
	}

	durations := make([]int64, 0, len(rows))
	ttfts := make([]int64, 0)
	modelStats := map[string]*appops.ModelTokenStat{}
	platStats := map[string]*appops.PlatformStat{}
	var errUpstream, errClient, errGateway, errOther int64

	for _, r := range rows {
		out.Summary.TotalRequests++
		if r.Success {
			out.Summary.SuccessRequests++
		} else {
			out.Summary.ErrorRequests++
			// 错误分类：上游 5xx 优先，其次客户端 4xx，其次网关 5xx，余下归 other。
			switch {
			case r.UpstreamStatusCode >= 500:
				errUpstream++
			case r.StatusCode >= 400 && r.StatusCode < 500:
				errClient++
			case r.StatusCode >= 500:
				errGateway++
			default:
				errOther++
			}
		}

		durations = append(durations, r.DurationMs)
		// 延迟直方图计数
		for i := range out.Histogram {
			b := &out.Histogram[i]
			if r.DurationMs >= b.Min && (b.Max == 0 || r.DurationMs < b.Max) {
				b.Count++
				break
			}
		}

		// TTFT 仅流式且有值才纳入
		if r.Stream && r.FirstTokenMs > 0 {
			ttfts = append(ttfts, r.FirstTokenMs)
		}

		// 按模型 token
		if r.Model != "" {
			ms, ok := modelStats[r.Model]
			if !ok {
				ms = &appops.ModelTokenStat{Model: r.Model}
				modelStats[r.Model] = ms
			}
			ms.Requests++
			ms.InputTokens += int64(r.InputTokens)
			ms.OutputTokens += int64(r.OutputTokens)
		}

		// 平台维度
		ps, ok := platStats[r.Platform]
		if !ok {
			ps = &appops.PlatformStat{Platform: r.Platform}
			platStats[r.Platform] = ps
		}
		ps.Requests++
		if !r.Success {
			ps.ErrorRequests++
		}
	}

	if out.Summary.TotalRequests > 0 {
		out.Summary.ErrorRate = float64(out.Summary.ErrorRequests) / float64(out.Summary.TotalRequests)
	}
	out.Summary.RPS = float64(out.Summary.TotalRequests) / float64(rangeSeconds)

	out.Latency = computePercentiles(durations)
	out.TTFT = computePercentiles(ttfts)

	// 错误分布（只列非零类）
	totalErr := out.Summary.ErrorRequests
	out.ErrorDistribution = buildErrorClasses(errUpstream, errClient, errGateway, errOther, totalErr)

	// 按模型 token（按请求数降序）
	out.TokensByModel = make([]appops.ModelTokenStat, 0, len(modelStats))
	for _, ms := range modelStats {
		out.TokensByModel = append(out.TokensByModel, *ms)
	}
	sort.Slice(out.TokensByModel, func(i, j int) bool {
		return out.TokensByModel[i].Requests > out.TokensByModel[j].Requests
	})

	// 平台维度（按请求数降序）
	out.PlatformBreakdown = make([]appops.PlatformStat, 0, len(platStats))
	for _, ps := range platStats {
		if ps.Requests > 0 {
			ps.ErrorRate = float64(ps.ErrorRequests) / float64(ps.Requests)
		}
		out.PlatformBreakdown = append(out.PlatformBreakdown, *ps)
	}
	sort.Slice(out.PlatformBreakdown, func(i, j int) bool {
		return out.PlatformBreakdown[i].Requests > out.PlatformBreakdown[j].Requests
	})

	return out, nil
}

// buildErrorClasses 把四类错误计数组装为非零的 ErrorClass 列表。
func buildErrorClasses(upstream, client, gateway, other, total int64) []appops.ErrorClass {
	defs := []struct {
		kind, label string
		count       int64
	}{
		{"upstream_5xx", "上游 5xx", upstream},
		{"client_4xx", "客户端 4xx", client},
		{"gateway_5xx", "网关 5xx", gateway},
		{"other", "其他", other},
	}
	out := make([]appops.ErrorClass, 0, len(defs))
	for _, d := range defs {
		if d.count == 0 {
			continue
		}
		ec := appops.ErrorClass{Kind: d.kind, Label: d.label, Count: d.count}
		if total > 0 {
			ec.Ratio = float64(d.count) / float64(total)
		}
		out = append(out, ec)
	}
	return out
}

// computePercentiles 从未排序切片算出 p50/p90/p95/p99/max/avg。
func computePercentiles(vals []int64) appops.Percentiles {
	n := len(vals)
	if n == 0 {
		return appops.Percentiles{}
	}
	sorted := make([]int64, n)
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var sum int64
	for _, v := range sorted {
		sum += v
	}
	return appops.Percentiles{
		P50:     percentile(sorted, 0.50),
		P90:     percentile(sorted, 0.90),
		P95:     percentile(sorted, 0.95),
		P99:     percentile(sorted, 0.99),
		Max:     sorted[n-1],
		Avg:     float64(sum) / float64(n),
		Samples: int64(n),
	}
}

// ListAccountConcurrency 读取所有账号的并发配置与状态（含所属分组名）。
func (s *OpsStore) ListAccountConcurrency(ctx context.Context) ([]appops.AccountConcurrencyInfo, error) {
	accounts, err := s.db.Account.Query().
		WithGroups().
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]appops.AccountConcurrencyInfo, 0, len(accounts))
	for _, a := range accounts {
		info := appops.AccountConcurrencyInfo{
			AccountID:      a.ID,
			Name:           a.Name,
			Platform:       a.Platform,
			MaxConcurrency: a.MaxConcurrency,
			State:          a.State.String(),
			StateUntil:     a.StateUntil,
		}
		for _, g := range a.Edges.Groups {
			info.Groups = append(info.Groups, g.Name)
		}
		out = append(out, info)
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

// ListWindowStats 查指定平台（platform=""=全平台汇总）最近 since 之后的窗口，按时间升序。
func (s *OpsStore) ListWindowStats(ctx context.Context, since time.Time, platform string) ([]appops.WindowStat, error) {
	rows, err := s.db.OpsWindowStat.Query().
		Where(
			entopswin.PlatformEQ(platform),
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

// InsertSystemLogs 批量写入系统日志。
func (s *OpsStore) InsertSystemLogs(ctx context.Context, entries []appops.SystemLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	builders := make([]*ent.OpsSystemLogCreate, 0, len(entries))
	for _, e := range entries {
		createdAt := e.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		builders = append(builders, s.db.OpsSystemLog.Create().
			SetLevel(e.Level).
			SetComponent(e.Component).
			SetMessage(e.Message).
			SetRequestID(e.RequestID).
			SetAttrs(e.Attrs).
			SetCreatedAt(createdAt))
	}
	_, err := s.db.OpsSystemLog.CreateBulk(builders...).Save(ctx)
	return err
}

// ListSystemLogs 分页查询系统日志。
func (s *OpsStore) ListSystemLogs(ctx context.Context, f appops.SystemLogFilter) (appops.SystemLogResult, error) {
	q := s.db.OpsSystemLog.Query()
	if f.Level != "" {
		q = q.Where(entopssyslog.LevelEQ(f.Level))
	}
	if f.Component != "" {
		q = q.Where(entopssyslog.ComponentEQ(f.Component))
	}
	if f.RequestID != "" {
		q = q.Where(entopssyslog.RequestIDEQ(f.RequestID))
	}
	if f.Keyword != "" {
		q = q.Where(entopssyslog.MessageContainsFold(f.Keyword))
	}
	if f.Start != nil {
		q = q.Where(entopssyslog.CreatedAtGTE(*f.Start))
	}
	if f.End != nil {
		q = q.Where(entopssyslog.CreatedAtLT(*f.End))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return appops.SystemLogResult{}, err
	}
	rows, err := q.
		Order(ent.Desc(entopssyslog.FieldCreatedAt)).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		All(ctx)
	if err != nil {
		return appops.SystemLogResult{}, err
	}
	list := make([]appops.SystemLogEntry, 0, len(rows))
	for _, r := range rows {
		list = append(list, appops.SystemLogEntry{
			ID:        r.ID,
			Level:     r.Level,
			Component: r.Component,
			Message:   r.Message,
			RequestID: r.RequestID,
			Attrs:     r.Attrs,
			CreatedAt: r.CreatedAt,
		})
	}
	return appops.SystemLogResult{
		List:     list,
		Total:    int64(total),
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

// PurgeSystemLogsBefore 删除 before 之前的系统日志。
func (s *OpsStore) PurgeSystemLogsBefore(ctx context.Context, before time.Time) (int, error) {
	return s.db.OpsSystemLog.Delete().
		Where(entopssyslog.CreatedAtLT(before)).
		Exec(ctx)
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
		ClientRequestID:    r.ClientRequestID,
		PluginID:           r.PluginID,
		Platform:           r.Platform,
		Model:              r.Model,
		UpstreamModel:      r.UpstreamModel,
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
