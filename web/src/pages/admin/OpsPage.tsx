import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { Card, useOverlayState } from '@heroui/react';
import {
  Activity,
  AlertTriangle,
  Gauge,
  Timer,
  RefreshCw,
  Maximize2,
  Minimize2,
  Cpu,
  MemoryStick,
  Database,
  HardDrive,
  Boxes,
  Recycle,
  CheckCircle2,
  XCircle,
  Zap,
} from 'lucide-react';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  CartesianGrid,
} from 'recharts';
import { opsApi } from '../../shared/api/ops';
import { OpsHealthCard } from './ops/OpsHealthCard';
import { OpsAlertPanel } from './ops/OpsAlertPanel';
import { OpsConcurrencyPanel } from './ops/OpsConcurrencyPanel';
import { OpsSystemLogPanel } from './ops/OpsSystemLogPanel';
import { OpsSettingsPanel } from './ops/OpsSettingsPanel';
import { CommonModal } from '../../shared/components/CommonModal';
import { usePlatforms } from '../../shared/hooks/usePlatforms';
import { useOpsSettings } from '../../shared/hooks/useOpsSettings';
import { useOpsStream } from '../../shared/hooks/useOpsStream';
import { queryKeys } from '../../shared/queryKeys';
import type { OpsRequestLogResp, OpsWindowStat } from '../../shared/types';

// 系统资源轮询间隔（ms）。资源变化快，固定较快刷新。
const SYSTEM_REFRESH_MS = 5000;
const PAGE_SIZE = 50;

// 自动刷新间隔选项（M16）。0=关闭。
const REFRESH_OPTIONS = [
  { key: 'off', label: '关闭', ms: 0 },
  { key: '5s', label: '5s', ms: 5000 },
  { key: '15s', label: '15s', ms: 15000 },
  { key: '30s', label: '30s', ms: 30000 },
  { key: '60s', label: '60s', ms: 60000 },
] as const;

// 时间范围选项（M2）。seconds 用于 analytics；分钟数用于趋势窗口。
const RANGES = [
  { key: '5m', label: '5 分钟', seconds: 300 },
  { key: '30m', label: '30 分钟', seconds: 1800 },
  { key: '1h', label: '1 小时', seconds: 3600 },
  { key: '6h', label: '6 小时', seconds: 21600 },
  { key: '24h', label: '24 小时', seconds: 86400 },
] as const;

// 错误分布饼图配色（按 kind 固定，便于辨识）。
const ERROR_COLORS: Record<string, string> = {
  upstream_5xx: '#ef4444',
  client_4xx: '#f59e0b',
  gateway_5xx: '#a855f7',
  other: '#64748b',
};

function fmtPercent(v: number): string {
  return `${(v * 100).toFixed(2)}%`;
}

function fmtNum(n: number): string {
  return n.toLocaleString();
}

// fmtBytes 将字节数格式化为人类可读（KB/MB/GB）。
function fmtBytes(n: number): string {
  if (!n || n < 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function fmtTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

function fmtClock(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

interface MetricCardProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  hint?: string;
  tone?: 'default' | 'danger' | 'warning' | 'success';
}

const TONE_VALUE_CLASS: Record<NonNullable<MetricCardProps['tone']>, string> = {
  default: 'text-foreground',
  danger: 'text-danger',
  warning: 'text-warning',
  success: 'text-success',
};

const TONE_ICON_CLASS: Record<NonNullable<MetricCardProps['tone']>, string> = {
  default: 'text-primary',
  danger: 'text-danger',
  warning: 'text-warning',
  success: 'text-success',
};

function MetricCard({ icon, label, value, hint, tone = 'default' }: MetricCardProps) {
  return (
    <Card className="min-h-[72px]">
      <div className="flex flex-row items-center justify-between gap-3 p-3.5">
        <div className="min-w-0 flex-1">
          <div className="text-xs text-default-500">{label}</div>
          <div className={`mt-1 truncate text-xl font-semibold ${TONE_VALUE_CLASS[tone]}`}>
            {value}
          </div>
          {hint && <div className="mt-0.5 truncate text-[11px] text-default-400">{hint}</div>}
        </div>
        <div className={TONE_ICON_CLASS[tone]}>{icon}</div>
      </div>
    </Card>
  );
}

// 区段标题
function SectionTitle({ icon, title, hint }: { icon?: React.ReactNode; title: string; hint?: string }) {
  return (
    <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-default-600">
      {icon}
      {title}
      {hint && <span className="text-[11px] font-normal text-default-400">({hint})</span>}
    </div>
  );
}

interface OpsSearch {
  range?: string;
  platform?: string;
  mode?: string;
}

export default function OpsPage() {
  const { t } = useTranslation();
  const { platforms } = usePlatforms();
  const { settings: opsSettings } = useOpsSettings();
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as OpsSearch;

  // ===== URL 持久化的筛选状态（M2）=====
  const rangeKey = RANGES.find((r) => r.key === search.range)?.key ?? '1h';
  const platform = search.platform ?? '';
  const mode = (search.mode as 'all' | 'success' | 'error') ?? 'error';
  const rangeSeconds = RANGES.find((r) => r.key === rangeKey)!.seconds;
  const trendWindows = Math.min(1440, Math.max(5, Math.round(rangeSeconds / 60)));

  const setSearch = useCallback(
    (patch: Partial<OpsSearch>) => {
      navigate({
        to: '/admin/ops',
        search: (prev: OpsSearch) => ({ ...prev, ...patch }),
        replace: true,
      });
    },
    [navigate],
  );

  // ===== 自动刷新间隔 + 全屏（M16）=====
  const [refreshKey, setRefreshKey] = useState<string>('15s');
  const refreshMs = REFRESH_OPTIONS.find((r) => r.key === refreshKey)?.ms ?? 15000;
  const refreshInterval: number | false = refreshMs > 0 ? refreshMs : false;
  const containerRef = useRef<HTMLDivElement>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  // 实时 SSE 推送（M5）：开关，默认开启。
  const [liveEnabled, setLiveEnabled] = useState(true);
  const { snapshot: live, status: liveStatus } = useOpsStream(liveEnabled);
  const toggleFullscreen = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    if (!document.fullscreenElement) {
      el.requestFullscreen?.().then(() => setIsFullscreen(true)).catch(() => {});
    } else {
      document.exitFullscreen?.().then(() => setIsFullscreen(false)).catch(() => {});
    }
  }, []);

  // ===== 趋势（按平台 + 时间范围）=====
  const overviewQuery = useQuery({
    queryKey: queryKeys.opsOverview(trendWindows, platform),
    queryFn: () => opsApi.overview({ trend_windows: trendWindows, platform: platform || undefined }),
    refetchInterval: refreshInterval,
    placeholderData: keepPreviousData,
  });
  const trend = overviewQuery.data?.trend ?? [];

  // ===== 分析（全分位/TTFT/直方图/错误分布/Token/平台维度）=====
  const analyticsQuery = useQuery({
    queryKey: queryKeys.opsAnalytics(rangeSeconds, platform),
    queryFn: () => opsApi.analytics({ range_seconds: rangeSeconds, platform: platform || undefined }),
    refetchInterval: refreshInterval,
    placeholderData: keepPreviousData,
  });
  const analytics = analyticsQuery.data;
  const summary = analytics?.summary;
  const latency = analytics?.latency;
  const ttft = analytics?.ttft;

  // ===== 系统资源（Core 自采，5s 轮询）=====
  const systemQuery = useQuery({
    queryKey: queryKeys.opsSystemMetrics(),
    queryFn: () => opsApi.systemMetrics(),
    refetchInterval: SYSTEM_REFRESH_MS,
    placeholderData: keepPreviousData,
  });
  const sys = systemQuery.data;
  const jobs = sys?.jobs ?? [];
  const jobErrors = jobs.filter((j) => j.last_error).length;

  // 趋势图数据
  const chartData = useMemo(
    () =>
      trend.map((w: OpsWindowStat) => ({
        time: fmtClock(w.window_start),
        rps: Number(w.rps.toFixed(2)),
        errorRate: Number((w.error_rate * 100).toFixed(2)),
        p95: w.p95_duration_ms,
      })),
    [trend],
  );

  // 直方图数据
  const histData = useMemo(
    () => (analytics?.histogram ?? []).map((b) => ({ label: b.label, count: b.count })),
    [analytics?.histogram],
  );

  // 错误分布饼图数据
  const errorPie = analytics?.error_distribution ?? [];

  // ===== 请求明细（M6）=====
  const [page, setPage] = useState(1);
  const [slowOnly, setSlowOnly] = useState(false);
  const [detail, setDetail] = useState<OpsRequestLogResp | null>(null);
  const detailModalState = useOverlayState({
    isOpen: detail !== null,
    onOpenChange: (open) => {
      if (!open) setDetail(null);
    },
  });

  // 请求链路（M12）：详情有 client_request_id 时查询关联尝试。
  const traceQuery = useQuery({
    queryKey: queryKeys.opsTrace(detail?.client_request_id ?? ''),
    queryFn: () => opsApi.trace(detail!.client_request_id),
    enabled: !!detail?.client_request_id,
  });
  const traceItems = traceQuery.data ?? [];

  const logsQuery = useQuery({
    queryKey: queryKeys.opsErrorLogs(page, platform, mode, rangeSeconds, slowOnly),
    queryFn: () =>
      opsApi.errorLogs({
        page,
        page_size: PAGE_SIZE,
        platform: platform || undefined,
        success_mode: mode,
        min_duration_ms: slowOnly ? 1000 : undefined,
        sort_by: slowOnly ? 'duration_ms' : 'created_at',
        sort_desc: 1,
        start: new Date(Date.now() - rangeSeconds * 1000).toISOString(),
      }),
    placeholderData: keepPreviousData,
  });

  const logs = logsQuery.data?.list ?? [];
  const total = logsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const handlePlatformChange = useCallback(
    (value: string) => {
      setSearch({ platform: value || undefined });
      setPage(1);
    },
    [setSearch],
  );

  const handleModeChange = useCallback(
    (value: 'all' | 'success' | 'error') => {
      setSearch({ mode: value });
      setPage(1);
    },
    [setSearch],
  );

  return (
    <div ref={containerRef} className="space-y-4 overflow-y-auto bg-background p-4">
      {/* 工具栏：标题 + 时间范围 + 平台 + 刷新指示 */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-lg font-semibold">{t('ops.title', { defaultValue: '运维监控' })}</h1>
        <div className="flex flex-wrap items-center gap-2">
          {/* 时间范围 */}
          <div className="flex overflow-hidden rounded-md border border-default-200">
            {RANGES.map((r) => (
              <button
                key={r.key}
                className={`px-2.5 py-1 text-xs ${
                  rangeKey === r.key
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-default-50 text-default-600 hover:bg-default-100'
                }`}
                onClick={() => {
                  setSearch({ range: r.key });
                  setPage(1);
                }}
              >
                {r.label}
              </button>
            ))}
          </div>
          {/* 平台 */}
          <select
            aria-label={t('ops.platform', { defaultValue: '平台' })}
            className="rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm"
            value={platform}
            onChange={(e) => handlePlatformChange(e.target.value)}
          >
            <option value="">{t('ops.all_platforms', { defaultValue: '全部平台' })}</option>
            {platforms.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
          {/* 实时推送（SSE）指示 + 开关 */}
          <button
            className={`flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs ${
              liveEnabled && liveStatus === 'live'
                ? 'border-success-200 bg-success-50 text-success-600'
                : liveEnabled && liveStatus === 'error'
                  ? 'border-danger-200 bg-danger-50 text-danger-600'
                  : 'border-default-200 bg-default-50 text-default-500'
            }`}
            title={t('ops.live_toggle', { defaultValue: '实时推送（SSE）' })}
            onClick={() => setLiveEnabled((v) => !v)}
          >
            <span
              className={`inline-block h-2 w-2 rounded-full ${
                liveEnabled && liveStatus === 'live'
                  ? 'animate-pulse bg-success'
                  : liveEnabled && liveStatus === 'error'
                    ? 'bg-danger'
                    : 'bg-default-300'
              }`}
            />
            {liveEnabled && live
              ? t('ops.live_rps', { defaultValue: '实时 {{rps}} RPS', rps: live.rps.toFixed(1) })
              : t('ops.live', { defaultValue: '实时' })}
          </button>

          {/* 自动刷新间隔 */}
          <div className="flex items-center gap-1 text-xs text-default-400">
            <RefreshCw
              className={`h-3.5 w-3.5 ${
                overviewQuery.isFetching || analyticsQuery.isFetching ? 'animate-spin' : ''
              }`}
            />
            <select
              aria-label={t('ops.refresh_interval', { defaultValue: '刷新间隔' })}
              className="rounded-md border border-default-200 bg-default-50 px-1.5 py-1 text-xs"
              value={refreshKey}
              onChange={(e) => setRefreshKey(e.target.value)}
            >
              {REFRESH_OPTIONS.map((r) => (
                <option key={r.key} value={r.key}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>
          {/* 全屏 */}
          <button
            className="rounded-md border border-default-200 bg-default-50 p-1.5 text-default-500 hover:bg-default-100"
            title={t('ops.fullscreen', { defaultValue: '全屏大屏' })}
            onClick={toggleFullscreen}
          >
            {isFullscreen ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
          </button>
        </div>
      </div>

      {summary?.sampled && (
        <div className="rounded-md bg-warning-50 px-3 py-1.5 text-xs text-warning-600">
          {t('ops.sampled_hint', {
            defaultValue: '当前时间范围数据量大，已按采样计算，分位为近似值。',
          })}
        </div>
      )}

      {/* 健康评分 + 诊断（M4）*/}
      <OpsHealthCard />

      {/* 概览指标卡（来自 analytics 区间汇总）*/}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <MetricCard
          icon={<Gauge className="h-6 w-6" />}
          label={t('ops.rps', { defaultValue: 'RPS（每秒请求）' })}
          value={summary ? summary.rps.toFixed(2) : '0'}
        />
        <MetricCard
          icon={<AlertTriangle className="h-6 w-6" />}
          label={t('ops.error_rate', { defaultValue: '错误率' })}
          value={summary ? fmtPercent(summary.error_rate) : '0%'}
          hint={summary ? `${fmtNum(summary.error_requests)} / ${fmtNum(summary.total_requests)}` : undefined}
          tone={
            summary && summary.error_rate >= opsSettings.errorRateCrit
              ? 'danger'
              : summary && summary.error_rate >= opsSettings.errorRateWarn
                ? 'warning'
                : 'default'
          }
        />
        <MetricCard
          icon={<Timer className="h-6 w-6" />}
          label={t('ops.p95', { defaultValue: 'P95 延迟' })}
          value={latency ? `${latency.p95} ms` : '0 ms'}
          tone={
            latency && latency.p95 >= opsSettings.p95Crit
              ? 'danger'
              : latency && latency.p95 >= opsSettings.p95Warn
                ? 'warning'
                : 'default'
          }
        />
        <MetricCard
          icon={<Activity className="h-6 w-6" />}
          label={t('ops.total_requests_range', { defaultValue: '区间总请求' })}
          value={summary ? fmtNum(summary.total_requests) : '0'}
        />
      </div>

      {/* 延迟分位（5 件套）+ TTFT */}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="p-3.5">
          <SectionTitle icon={<Timer className="h-4 w-4" />} title={t('ops.latency_quantiles', { defaultValue: '延迟分位' })} />
          <div className="grid grid-cols-5 gap-2 text-center">
            {([
              ['P50', latency?.p50],
              ['P90', latency?.p90],
              ['P95', latency?.p95],
              ['P99', latency?.p99],
              ['Max', latency?.max],
            ] as const).map(([k, v]) => (
              <div key={k} className="rounded-md bg-default-50 py-2">
                <div className="text-[11px] text-default-400">{k}</div>
                <div className="text-base font-semibold">{v ?? 0}</div>
                <div className="text-[10px] text-default-300">ms</div>
              </div>
            ))}
          </div>
          <div className="mt-2 text-[11px] text-default-400">
            {t('ops.avg', { defaultValue: '平均' })} {latency ? latency.avg.toFixed(0) : 0} ms ·{' '}
            {fmtNum(latency?.samples ?? 0)} {t('ops.samples', { defaultValue: '样本' })}
          </div>
        </Card>
        <Card className="p-3.5">
          <SectionTitle
            icon={<Zap className="h-4 w-4" />}
            title={t('ops.ttft', { defaultValue: '首 Token 延迟（TTFT）' })}
            hint={t('ops.ttft_hint', { defaultValue: '仅流式' })}
          />
          {ttft && ttft.samples > 0 ? (
            <>
              <div className="grid grid-cols-5 gap-2 text-center">
                {([
                  ['P50', ttft.p50],
                  ['P90', ttft.p90],
                  ['P95', ttft.p95],
                  ['P99', ttft.p99],
                  ['Max', ttft.max],
                ] as const).map(([k, v]) => (
                  <div key={k} className="rounded-md bg-default-50 py-2">
                    <div className="text-[11px] text-default-400">{k}</div>
                    <div className="text-base font-semibold">{v}</div>
                    <div className="text-[10px] text-default-300">ms</div>
                  </div>
                ))}
              </div>
              <div className="mt-2 text-[11px] text-default-400">
                {t('ops.avg', { defaultValue: '平均' })} {ttft.avg.toFixed(0)} ms ·{' '}
                {fmtNum(ttft.samples)} {t('ops.samples', { defaultValue: '样本' })}
              </div>
            </>
          ) : (
            <div className="flex h-[88px] items-center justify-center text-sm text-default-400">
              {t('ops.no_ttft', { defaultValue: '暂无流式请求 TTFT 数据' })}
            </div>
          )}
        </Card>
      </div>

      {/* 系统资源（Core 进程自采）*/}
      <div>
        <SectionTitle
          icon={<Cpu className="h-4 w-4" />}
          title={t('ops.system_title', { defaultValue: '系统资源' })}
          hint={t('ops.system_hint', { defaultValue: 'Core 进程，5s 刷新' })}
        />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-3 xl:grid-cols-6">
          <MetricCard
            icon={<Boxes className="h-6 w-6" />}
            label={t('ops.goroutines', { defaultValue: 'Goroutines' })}
            value={sys ? String(sys.goroutines) : '-'}
            tone={sys && sys.goroutines > opsSettings.goroutineWarn ? 'danger' : 'default'}
          />
          <MetricCard
            icon={<MemoryStick className="h-6 w-6" />}
            label={t('ops.heap_alloc', { defaultValue: '堆内存（已用）' })}
            value={sys ? fmtBytes(sys.memory.heap_alloc_bytes) : '-'}
            hint={
              sys
                ? t('ops.sys_mem_hint', {
                    defaultValue: '系统申请 {{sys}}',
                    sys: fmtBytes(sys.memory.sys_bytes),
                  })
                : undefined
            }
          />
          <MetricCard
            icon={<Recycle className="h-6 w-6" />}
            label={t('ops.num_gc', { defaultValue: 'GC 次数' })}
            value={sys ? String(sys.memory.num_gc) : '-'}
          />
          <MetricCard
            icon={<HardDrive className="h-6 w-6" />}
            label={t('ops.db_pool', { defaultValue: 'DB 连接池' })}
            value={
              sys
                ? `${sys.db.in_use} / ${sys.db.max_open_conns > 0 ? sys.db.max_open_conns : '∞'}`
                : '-'
            }
            hint={
              sys
                ? t('ops.db_pool_hint', {
                    defaultValue: '空闲 {{idle}} · 等待 {{wait}}',
                    idle: sys.db.idle,
                    wait: sys.db.wait_count,
                  })
                : undefined
            }
            tone={
              sys && sys.db.max_open_conns > 0 && sys.db.in_use >= sys.db.max_open_conns
                ? 'danger'
                : sys && sys.db.wait_count > 0
                  ? 'warning'
                  : 'default'
            }
          />
          <MetricCard
            icon={<Database className="h-6 w-6" />}
            label={t('ops.redis_conns', { defaultValue: 'Redis 连接' })}
            value={sys ? String(sys.redis.total_conns) : '-'}
            hint={
              sys
                ? t('ops.redis_conns_hint', {
                    defaultValue: '空闲 {{idle}} · 超时 {{timeouts}}',
                    idle: sys.redis.idle_conns,
                    timeouts: sys.redis.timeouts,
                  })
                : undefined
            }
            tone={sys && sys.redis.timeouts > 0 ? 'danger' : 'default'}
          />
          <MetricCard
            icon={jobErrors > 0 ? <XCircle className="h-6 w-6" /> : <CheckCircle2 className="h-6 w-6" />}
            label={t('ops.jobs', { defaultValue: '后台任务' })}
            value={
              jobs.length === 0
                ? '-'
                : jobErrors > 0
                  ? t('ops.jobs_error', { defaultValue: '{{n}} 异常', n: jobErrors })
                  : t('ops.jobs_ok', { defaultValue: '{{n}} 正常', n: jobs.length })
            }
            tone={jobErrors > 0 ? 'danger' : jobs.length > 0 ? 'success' : 'default'}
          />
        </div>

        {/* 后台任务明细 */}
        {jobs.length > 0 && (
          <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-2 lg:grid-cols-3">
            {jobs.map((job) => (
              <div
                key={job.name}
                className="flex items-center justify-between gap-2 rounded-md border border-default-200 bg-default-50 px-3 py-2 text-xs"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5 font-medium text-default-700">
                    {job.last_error ? (
                      <XCircle className="h-3.5 w-3.5 text-danger" />
                    ) : (
                      <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    )}
                    {job.name}
                  </div>
                  {job.last_error ? (
                    <div className="mt-0.5 truncate text-danger" title={job.last_error}>
                      {job.last_error}
                    </div>
                  ) : (
                    <div className="mt-0.5 text-default-400">
                      {t('ops.job_last_run', { defaultValue: '上次运行' })}：
                      {job.last_run_at ? fmtClock(job.last_run_at) : '-'}
                    </div>
                  )}
                </div>
                <div className="shrink-0 text-right text-default-400">
                  <div>{t('ops.job_heartbeat', { defaultValue: '心跳' })}</div>
                  <div>{job.last_heartbeat ? fmtClock(job.last_heartbeat) : '-'}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 告警（M3）*/}
      <OpsAlertPanel />

      {/* 并发与账号可用性（M7）*/}
      <OpsConcurrencyPanel />

      {/* 趋势图 */}
      <Card className="p-4">
        <SectionTitle title={t('ops.trend', { defaultValue: '吞吐 / 错误率趋势' })} />
        {chartData.length === 0 ? (
          <div className="flex h-[240px] items-center justify-center text-sm text-default-400">
            {t('ops.no_data', { defaultValue: '暂无数据（发起请求后约 1 分钟出现）' })}
          </div>
        ) : (
          <div className="h-[240px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 4, right: 8, left: -18, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--heroui-default-200))" />
                <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Line type="monotone" dataKey="rps" name="RPS" stroke="#3b82f6" dot={false} strokeWidth={2} />
                <Line
                  type="monotone"
                  dataKey="errorRate"
                  name={t('ops.error_rate', { defaultValue: '错误率' }) + ' %'}
                  stroke="#ef4444"
                  dot={false}
                  strokeWidth={2}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </Card>

      {/* 延迟直方图 + 错误分布 */}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="p-4">
          <SectionTitle title={t('ops.latency_histogram', { defaultValue: '延迟分布' })} />
          {histData.every((b) => b.count === 0) ? (
            <div className="flex h-[220px] items-center justify-center text-sm text-default-400">
              {t('ops.no_data', { defaultValue: '暂无数据' })}
            </div>
          ) : (
            <div className="h-[220px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={histData} margin={{ top: 4, right: 8, left: -18, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--heroui-default-200))" />
                  <XAxis dataKey="label" tick={{ fontSize: 10 }} interval={0} angle={-20} textAnchor="end" height={48} />
                  <YAxis tick={{ fontSize: 11 }} allowDecimals={false} />
                  <Tooltip />
                  <Bar dataKey="count" name={t('ops.requests', { defaultValue: '请求数' })} fill="#3b82f6" radius={[3, 3, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </Card>
        <Card className="p-4">
          <SectionTitle title={t('ops.error_distribution', { defaultValue: '错误分布' })} />
          {errorPie.length === 0 ? (
            <div className="flex h-[220px] items-center justify-center text-sm text-default-400">
              {t('ops.no_errors', { defaultValue: '暂无错误' })}
            </div>
          ) : (
            <div className="h-[220px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={errorPie.map((e) => ({ name: e.label, value: e.count, kind: e.kind }))}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="50%"
                    innerRadius={48}
                    outerRadius={78}
                    paddingAngle={2}
                  >
                    {errorPie.map((e) => (
                      <Cell key={e.kind} fill={ERROR_COLORS[e.kind] ?? '#64748b'} />
                    ))}
                  </Pie>
                  <Tooltip />
                  <Legend wrapperStyle={{ fontSize: 11 }} />
                </PieChart>
              </ResponsiveContainer>
            </div>
          )}
        </Card>
      </div>

      {/* 平台维度 + 按模型 Token */}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="p-4">
          <SectionTitle title={t('ops.platform_breakdown', { defaultValue: '平台维度' })} />
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-default-200 text-left text-xs text-default-500">
                  <th className="px-2 py-1.5">{t('ops.col_platform', { defaultValue: '平台' })}</th>
                  <th className="px-2 py-1.5 text-right">{t('ops.requests', { defaultValue: '请求数' })}</th>
                  <th className="px-2 py-1.5 text-right">{t('ops.error_rate', { defaultValue: '错误率' })}</th>
                </tr>
              </thead>
              <tbody>
                {(analytics?.platform_breakdown ?? []).length === 0 ? (
                  <tr>
                    <td colSpan={3} className="px-2 py-6 text-center text-default-400">
                      {t('ops.no_data', { defaultValue: '暂无数据' })}
                    </td>
                  </tr>
                ) : (
                  (analytics?.platform_breakdown ?? []).map((p) => (
                    <tr key={p.platform || '_'} className="border-b border-default-100">
                      <td className="px-2 py-1.5">{p.platform || '-'}</td>
                      <td className="px-2 py-1.5 text-right">{fmtNum(p.requests)}</td>
                      <td className={`px-2 py-1.5 text-right ${p.error_rate > 0 ? 'text-danger' : 'text-default-500'}`}>
                        {fmtPercent(p.error_rate)}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </Card>
        <Card className="p-4">
          <SectionTitle title={t('ops.tokens_by_model', { defaultValue: '按模型 Token' })} />
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-default-200 text-left text-xs text-default-500">
                  <th className="px-2 py-1.5">{t('ops.col_model', { defaultValue: '模型' })}</th>
                  <th className="px-2 py-1.5 text-right">{t('ops.requests', { defaultValue: '请求数' })}</th>
                  <th className="px-2 py-1.5 text-right">{t('ops.input_tokens', { defaultValue: '输入' })}</th>
                  <th className="px-2 py-1.5 text-right">{t('ops.output_tokens', { defaultValue: '输出' })}</th>
                </tr>
              </thead>
              <tbody>
                {(analytics?.tokens_by_model ?? []).length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-2 py-6 text-center text-default-400">
                      {t('ops.no_data', { defaultValue: '暂无数据' })}
                    </td>
                  </tr>
                ) : (
                  (analytics?.tokens_by_model ?? []).slice(0, 12).map((m) => (
                    <tr key={m.model} className="border-b border-default-100">
                      <td className="max-w-[180px] truncate px-2 py-1.5" title={m.model}>
                        {m.model}
                      </td>
                      <td className="px-2 py-1.5 text-right">{fmtNum(m.requests)}</td>
                      <td className="px-2 py-1.5 text-right text-default-500">{fmtNum(m.input_tokens)}</td>
                      <td className="px-2 py-1.5 text-right text-default-500">{fmtNum(m.output_tokens)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </Card>
      </div>

      {/* 请求明细（M6）*/}
      <Card className="p-4">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div className="text-sm font-medium text-default-600">
            {t('ops.request_logs', { defaultValue: '请求明细' })}
          </div>
          <div className="flex items-center gap-2">
            {/* success 模式切换 */}
            <div className="flex overflow-hidden rounded-md border border-default-200">
              {([
                ['error', t('ops.mode_error', { defaultValue: '错误' })],
                ['success', t('ops.mode_success', { defaultValue: '成功' })],
                ['all', t('ops.mode_all', { defaultValue: '全部' })],
              ] as const).map(([k, label]) => (
                <button
                  key={k}
                  className={`px-2.5 py-1 text-xs ${
                    mode === k
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-default-50 text-default-600 hover:bg-default-100'
                  }`}
                  onClick={() => handleModeChange(k)}
                >
                  {label}
                </button>
              ))}
            </div>
            {/* 慢请求 */}
            <button
              className={`rounded-md border px-2.5 py-1 text-xs ${
                slowOnly
                  ? 'border-warning bg-warning-50 text-warning-600'
                  : 'border-default-200 bg-default-50 text-default-600 hover:bg-default-100'
              }`}
              onClick={() => {
                setSlowOnly((v) => !v);
                setPage(1);
              }}
            >
              {t('ops.slow_only', { defaultValue: '慢请求 >1s' })}
            </button>
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-default-200 text-left text-xs text-default-500">
                <th className="px-2 py-2">{t('ops.col_time', { defaultValue: '时间' })}</th>
                <th className="px-2 py-2">{t('ops.col_platform', { defaultValue: '平台' })}</th>
                <th className="px-2 py-2">{t('ops.col_model', { defaultValue: '模型' })}</th>
                <th className="px-2 py-2">{t('ops.col_status', { defaultValue: '状态码' })}</th>
                <th className="px-2 py-2">{t('ops.col_kind', { defaultValue: '错误类型' })}</th>
                <th className="px-2 py-2">{t('ops.col_duration', { defaultValue: '耗时' })}</th>
                <th className="px-2 py-2">{t('ops.col_msg', { defaultValue: '信息' })}</th>
              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-2 py-8 text-center text-default-400">
                    {t('ops.no_logs', { defaultValue: '暂无记录' })}
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr
                    key={log.id}
                    className="cursor-pointer border-b border-default-100 hover:bg-default-50"
                    onClick={() => setDetail(log)}
                  >
                    <td className="whitespace-nowrap px-2 py-2 text-default-600">{fmtTime(log.created_at)}</td>
                    <td className="px-2 py-2">{log.platform || '-'}</td>
                    <td className="max-w-[140px] truncate px-2 py-2" title={log.model}>
                      {log.model || '-'}
                    </td>
                    <td className="px-2 py-2">
                      <span className={log.success ? 'text-success' : 'text-danger'}>
                        {log.status_code || '-'}
                      </span>
                    </td>
                    <td className="px-2 py-2">{log.error_kind || '-'}</td>
                    <td className="whitespace-nowrap px-2 py-2 text-default-500">{log.duration_ms} ms</td>
                    <td className="max-w-[260px] truncate px-2 py-2 text-default-500">
                      {log.success ? '-' : log.error_msg || '-'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="mt-3 flex items-center justify-end gap-2 text-sm">
            <button
              className="rounded px-2 py-1 disabled:opacity-40"
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              {t('common.prev', { defaultValue: '上一页' })}
            </button>
            <span className="text-default-500">
              {page} / {totalPages}
            </span>
            <button
              className="rounded px-2 py-1 disabled:opacity-40"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            >
              {t('common.next', { defaultValue: '下一页' })}
            </button>
          </div>
        )}
      </Card>

      {/* 系统日志（M11）*/}
      <OpsSystemLogPanel />

      {/* 运维设置（M15）*/}
      <OpsSettingsPanel />

      {/* 详情弹窗 */}
      <CommonModal
        state={detailModalState}
        title={t('ops.detail_title', { defaultValue: '请求详情' })}
        dialogStyle={{ maxWidth: '640px', width: 'min(100%, calc(100vw - 2rem))' }}
      >
        {detail && (
          <div className="space-y-2 text-sm">
            <DetailRow label="Request ID" value={detail.request_id || '-'} />
            <DetailRow label={t('ops.col_platform', { defaultValue: '平台' })} value={detail.platform} />
            <DetailRow
              label={t('ops.col_model', { defaultValue: '模型' })}
              value={
                detail.upstream_model && detail.upstream_model !== detail.model
                  ? `${detail.model} → ${detail.upstream_model}`
                  : detail.model
              }
            />
            <DetailRow label="Endpoint" value={detail.endpoint || '-'} />
            <DetailRow label={t('ops.col_status', { defaultValue: '状态码' })} value={String(detail.status_code)} />
            <DetailRow
              label={t('ops.upstream_status', { defaultValue: '上游状态码' })}
              value={String(detail.upstream_status_code)}
            />
            <DetailRow label={t('ops.col_kind', { defaultValue: '错误类型' })} value={detail.error_kind || '-'} />
            <DetailRow label={t('ops.col_duration', { defaultValue: '耗时' })} value={`${detail.duration_ms} ms`} />
            <DetailRow label="TTFT" value={detail.first_token_ms ? `${detail.first_token_ms} ms` : '-'} />
            <DetailRow
              label="Token"
              value={`${t('ops.input_tokens', { defaultValue: '输入' })} ${detail.input_tokens} / ${t('ops.output_tokens', { defaultValue: '输出' })} ${detail.output_tokens}`}
            />
            <DetailRow label={t('ops.account_id', { defaultValue: '账号 ID' })} value={String(detail.account_id)} />
            {!detail.success && (
              <div>
                <div className="mb-1 text-xs text-default-500">{t('ops.col_msg', { defaultValue: '错误信息' })}</div>
                <div className="rounded bg-default-100 p-2 text-default-700">{detail.error_msg || '-'}</div>
              </div>
            )}
            {detail.error_detail && (
              <div>
                <div className="mb-1 text-xs text-default-500">
                  {t('ops.error_detail', { defaultValue: '详细 / 上游响应' })}
                </div>
                <pre className="max-h-64 overflow-auto rounded bg-default-100 p-2 text-xs text-default-700">
                  {detail.error_detail}
                </pre>
              </div>
            )}

            {/* 请求链路（M12 trace）：同 client_request_id 的多次尝试 */}
            {detail.client_request_id && traceItems.length > 1 && (
              <div>
                <div className="mb-1 text-xs text-default-500">
                  {t('ops.trace_title', { defaultValue: '请求链路（{{n}} 次尝试）', n: traceItems.length })}
                </div>
                <div className="space-y-1">
                  {traceItems.map((it, i) => (
                    <div
                      key={it.id}
                      className={`flex items-center gap-2 rounded px-2 py-1 text-xs ${
                        it.id === detail.id ? 'bg-primary-50' : 'bg-default-50'
                      }`}
                    >
                      <span className="text-default-400">#{i + 1}</span>
                      <span className={it.success ? 'text-success' : 'text-danger'}>
                        {it.success ? '✓' : '✗'} {it.status_code}
                      </span>
                      <span className="text-default-500">{it.platform}</span>
                      <span className="text-default-400">acct {it.account_id}</span>
                      <span className="ml-auto text-default-400">{it.duration_ms} ms</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </CommonModal>
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-3">
      <div className="w-28 shrink-0 text-default-500">{label}</div>
      <div className="min-w-0 flex-1 break-all text-default-700">{value}</div>
    </div>
  );
}
