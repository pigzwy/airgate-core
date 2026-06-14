import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Card, useOverlayState } from '@heroui/react';
import {
  Activity,
  AlertTriangle,
  Gauge,
  Timer,
  RefreshCw,
} from 'lucide-react';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import { opsApi } from '../../shared/api/ops';
import { CommonModal } from '../../shared/components/CommonModal';
import { usePlatforms } from '../../shared/hooks/usePlatforms';
import { queryKeys } from '../../shared/queryKeys';
import type { OpsRequestLogResp, OpsWindowStat } from '../../shared/types';

// 大盘轮询间隔（ms）。MVP 用轮询，后续可换 WebSocket。
const OVERVIEW_REFRESH_MS = 15000;
const PAGE_SIZE = 50;

function fmtPercent(v: number): string {
  return `${(v * 100).toFixed(2)}%`;
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
  tone?: 'default' | 'danger';
}

function MetricCard({ icon, label, value, tone = 'default' }: MetricCardProps) {
  return (
    <Card className="min-h-[72px]">
      <div className="flex flex-row items-center justify-between gap-3 p-3.5">
        <div className="min-w-0 flex-1">
          <div className="text-xs text-default-500">{label}</div>
          <div
            className={`mt-1 truncate text-xl font-semibold ${
              tone === 'danger' ? 'text-danger' : 'text-foreground'
            }`}
          >
            {value}
          </div>
        </div>
        <div className={tone === 'danger' ? 'text-danger' : 'text-primary'}>{icon}</div>
      </div>
    </Card>
  );
}

export default function OpsPage() {
  const { t } = useTranslation();
  const { platforms } = usePlatforms();

  // ===== 实时大盘 =====
  const overviewQuery = useQuery({
    queryKey: queryKeys.opsOverview(),
    queryFn: () => opsApi.overview({ trend_windows: 60 }),
    refetchInterval: OVERVIEW_REFRESH_MS,
    placeholderData: keepPreviousData,
  });

  const latest: OpsWindowStat | undefined = overviewQuery.data?.latest;
  const trend = overviewQuery.data?.trend ?? [];

  const chartData = useMemo(
    () =>
      trend.map((w) => ({
        time: fmtClock(w.window_start),
        rps: Number(w.rps.toFixed(2)),
        errorRate: Number((w.error_rate * 100).toFixed(2)),
        p95: w.p95_duration_ms,
      })),
    [trend],
  );

  // ===== 错误日志 =====
  const [page, setPage] = useState(1);
  const [platform, setPlatform] = useState('');
  const [detail, setDetail] = useState<OpsRequestLogResp | null>(null);
  const detailModalState = useOverlayState({
    isOpen: detail !== null,
    onOpenChange: (open) => {
      if (!open) setDetail(null);
    },
  });

  const logsQuery = useQuery({
    queryKey: queryKeys.opsErrorLogs(page, platform),
    queryFn: () =>
      opsApi.errorLogs({
        page,
        page_size: PAGE_SIZE,
        platform: platform || undefined,
        only_errors: 1,
      }),
    placeholderData: keepPreviousData,
  });

  const logs = logsQuery.data?.list ?? [];
  const total = logsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const handlePlatformChange = useCallback((value: string) => {
    setPlatform(value);
    setPage(1);
  }, []);

  return (
    <div className="space-y-4 p-4">
      {/* 标题 + 刷新状态 */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">{t('ops.title', { defaultValue: '运维监控' })}</h1>
        <div className="flex items-center gap-1.5 text-xs text-default-400">
          <RefreshCw className={`h-3.5 w-3.5 ${overviewQuery.isFetching ? 'animate-spin' : ''}`} />
          {t('ops.auto_refresh', { defaultValue: '自动刷新' })}
        </div>
      </div>

      {/* 指标卡 */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <MetricCard
          icon={<Gauge className="h-6 w-6" />}
          label={t('ops.rps', { defaultValue: 'RPS（每秒请求）' })}
          value={latest ? latest.rps.toFixed(2) : '0'}
        />
        <MetricCard
          icon={<AlertTriangle className="h-6 w-6" />}
          label={t('ops.error_rate', { defaultValue: '错误率' })}
          value={latest ? fmtPercent(latest.error_rate) : '0%'}
          tone={latest && latest.error_rate > 0 ? 'danger' : 'default'}
        />
        <MetricCard
          icon={<Timer className="h-6 w-6" />}
          label={t('ops.p95', { defaultValue: 'P95 延迟' })}
          value={latest ? `${latest.p95_duration_ms} ms` : '0 ms'}
        />
        <MetricCard
          icon={<Activity className="h-6 w-6" />}
          label={t('ops.total_requests', { defaultValue: '窗口请求数' })}
          value={latest ? String(latest.total_requests) : '0'}
        />
      </div>

      {/* 趋势图 */}
      <Card className="p-4">
        <div className="mb-2 text-sm font-medium text-default-600">
          {t('ops.trend', { defaultValue: '近 1 小时趋势' })}
        </div>
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
                <Line
                  type="monotone"
                  dataKey="rps"
                  name="RPS"
                  stroke="#3b82f6"
                  dot={false}
                  strokeWidth={2}
                />
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

      {/* 错误日志 */}
      <Card className="p-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="text-sm font-medium text-default-600">
            {t('ops.error_logs', { defaultValue: '错误日志' })}
          </div>
          <div className="w-44">
            <select
              aria-label={t('ops.platform', { defaultValue: '平台' })}
              className="w-full rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm"
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
                <th className="px-2 py-2">{t('ops.col_msg', { defaultValue: '错误信息' })}</th>
              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-2 py-8 text-center text-default-400">
                    {t('ops.no_errors', { defaultValue: '暂无错误日志' })}
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr
                    key={log.id}
                    className="cursor-pointer border-b border-default-100 hover:bg-default-50"
                    onClick={() => setDetail(log)}
                  >
                    <td className="whitespace-nowrap px-2 py-2 text-default-600">
                      {fmtTime(log.created_at)}
                    </td>
                    <td className="px-2 py-2">{log.platform || '-'}</td>
                    <td className="px-2 py-2">{log.model || '-'}</td>
                    <td className="px-2 py-2">
                      <span className="text-danger">{log.status_code || '-'}</span>
                    </td>
                    <td className="px-2 py-2">{log.error_kind || '-'}</td>
                    <td className="whitespace-nowrap px-2 py-2 text-default-500">
                      {log.duration_ms} ms
                    </td>
                    <td className="max-w-[280px] truncate px-2 py-2 text-default-500">
                      {log.error_msg || '-'}
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
                <DetailRow label={t('ops.col_model', { defaultValue: '模型' })} value={detail.model} />
                <DetailRow label="Endpoint" value={detail.endpoint || '-'} />
                <DetailRow label={t('ops.col_status', { defaultValue: '状态码' })} value={String(detail.status_code)} />
                <DetailRow
                  label={t('ops.upstream_status', { defaultValue: '上游状态码' })}
                  value={String(detail.upstream_status_code)}
                />
                <DetailRow label={t('ops.col_kind', { defaultValue: '错误类型' })} value={detail.error_kind || '-'} />
                <DetailRow label={t('ops.col_duration', { defaultValue: '耗时' })} value={`${detail.duration_ms} ms`} />
                <DetailRow label={t('ops.account_id', { defaultValue: '账号 ID' })} value={String(detail.account_id)} />
                <div>
                  <div className="mb-1 text-xs text-default-500">
                    {t('ops.col_msg', { defaultValue: '错误信息' })}
                  </div>
                  <div className="rounded bg-default-100 p-2 text-default-700">
                    {detail.error_msg || '-'}
                  </div>
                </div>
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
