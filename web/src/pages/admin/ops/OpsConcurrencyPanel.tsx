import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { Users, Shuffle } from 'lucide-react';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import { opsApi } from '../../../shared/api/ops';
import { queryKeys } from '../../../shared/queryKeys';
import type { OpsConcurrencyItem } from '../../../shared/types';

const REFRESH_MS = 5000;

// 账号状态徽章配色
const STATE_BADGE: Record<string, string> = {
  active: 'bg-success-100 text-success-700',
  rate_limited: 'bg-warning-100 text-warning-700',
  degraded: 'bg-secondary-100 text-secondary-700',
  disabled: 'bg-danger-100 text-danger-700',
};

function usageColor(usage: number): string {
  if (usage >= 0.9) return 'bg-danger';
  if (usage >= 0.7) return 'bg-warning';
  return 'bg-primary';
}

function UsageBar({ current, max, usage }: { current: number; max: number; usage: number }) {
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-24 overflow-hidden rounded-full bg-default-200">
        <div
          className={`h-full ${usageColor(usage)}`}
          style={{ width: `${Math.min(100, Math.round(usage * 100))}%` }}
        />
      </div>
      <span className="text-xs text-default-500">
        {current}/{max > 0 ? max : '∞'}
      </span>
    </div>
  );
}

function fmtCountdown(seconds: number): string {
  if (seconds <= 0) return '-';
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}m${s > 0 ? `${s}s` : ''}`;
}

function ItemList({ items }: { items: OpsConcurrencyItem[] }) {
  const { t } = useTranslation();
  if (items.length === 0) {
    return (
      <div className="py-6 text-center text-sm text-default-400">
        {t('ops.no_data', { defaultValue: '暂无数据' })}
      </div>
    );
  }
  return (
    <div className="space-y-2">
      {items.map((it) => (
        <div key={it.key || '_'} className="flex items-center justify-between gap-3">
          <div className="min-w-0 flex-1 truncate text-sm text-default-700" title={it.key}>
            {it.key || '-'}
          </div>
          <UsageBar current={it.current} max={it.max} usage={it.usage} />
        </div>
      ))}
    </div>
  );
}

export function OpsConcurrencyPanel() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<'account' | 'platform' | 'group'>('account');

  const query = useQuery({
    queryKey: queryKeys.opsConcurrency(),
    queryFn: () => opsApi.concurrency(),
    refetchInterval: REFRESH_MS,
    placeholderData: keepPreviousData,
  });
  const data = query.data;
  const avail = data?.availability;

  // 切换率趋势（最近 60 分钟）
  const switchQuery = useQuery({
    queryKey: queryKeys.opsSwitchRate(60),
    queryFn: () => opsApi.switchRate(60),
    refetchInterval: 30000,
    placeholderData: keepPreviousData,
  });
  const switchData = (switchQuery.data ?? []).map((p) => ({
    time: new Date(p.minute).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    rate: Number((p.switch_rate * 100).toFixed(1)),
    switches: p.switches,
  }));
  const hasSwitchData = switchData.some((p) => p.switches > 0);

  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-default-600">
        <Users className="h-4 w-4" />
        {t('ops.concurrency_title', { defaultValue: '并发与账号可用性' })}
        {data && (
          <span className="text-[11px] font-normal text-default-400">
            ({t('ops.concurrency_total', {
              defaultValue: '总并发 {{cur}}/{{max}}',
              cur: data.total_current,
              max: data.total_max,
            })})
          </span>
        )}
      </div>

      {/* 账号可用性汇总 */}
      <div className="mb-3 grid grid-cols-2 gap-3 md:grid-cols-4">
        <AvailCard label={t('ops.state_active', { defaultValue: '活跃' })} value={avail?.active ?? 0} tone="success" />
        <AvailCard
          label={t('ops.state_rate_limited', { defaultValue: '被限流' })}
          value={avail?.rate_limited ?? 0}
          tone="warning"
          hint={
            avail && avail.soonest_recovery_seconds > 0
              ? t('ops.recovery_in', {
                  defaultValue: '最快 {{t}} 恢复',
                  t: fmtCountdown(avail.soonest_recovery_seconds),
                })
              : undefined
          }
        />
        <AvailCard label={t('ops.state_degraded', { defaultValue: '降级' })} value={avail?.degraded ?? 0} tone="secondary" />
        <AvailCard label={t('ops.state_disabled', { defaultValue: '禁用' })} value={avail?.disabled ?? 0} tone="danger" />
      </div>

      <Card className="p-4">
        {/* 维度切换 */}
        <div className="mb-3 flex overflow-hidden rounded-md border border-default-200 w-fit">
          {([
            ['account', t('ops.dim_account', { defaultValue: '账号' })],
            ['platform', t('ops.dim_platform', { defaultValue: '平台' })],
            ['group', t('ops.dim_group', { defaultValue: '分组' })],
          ] as const).map(([k, label]) => (
            <button
              key={k}
              className={`px-3 py-1 text-xs ${
                tab === k
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-default-50 text-default-600 hover:bg-default-100'
              }`}
              onClick={() => setTab(k)}
            >
              {label}
            </button>
          ))}
        </div>

        {tab === 'account' ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-default-200 text-left text-xs text-default-500">
                  <th className="px-2 py-1.5">{t('ops.col_account', { defaultValue: '账号' })}</th>
                  <th className="px-2 py-1.5">{t('ops.col_platform', { defaultValue: '平台' })}</th>
                  <th className="px-2 py-1.5">{t('ops.col_concurrency', { defaultValue: '并发' })}</th>
                  <th className="px-2 py-1.5">{t('ops.col_state', { defaultValue: '状态' })}</th>
                  <th className="px-2 py-1.5">{t('ops.col_recovery', { defaultValue: '恢复' })}</th>
                </tr>
              </thead>
              <tbody>
                {(data?.by_account ?? []).length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-2 py-6 text-center text-default-400">
                      {t('ops.no_data', { defaultValue: '暂无数据' })}
                    </td>
                  </tr>
                ) : (
                  (data?.by_account ?? []).map((a) => (
                    <tr key={a.account_id} className="border-b border-default-100">
                      <td className="max-w-[160px] truncate px-2 py-1.5" title={a.name}>
                        {a.name}
                      </td>
                      <td className="px-2 py-1.5 text-default-500">{a.platform}</td>
                      <td className="px-2 py-1.5">
                        <UsageBar current={a.current} max={a.max} usage={a.usage} />
                      </td>
                      <td className="px-2 py-1.5">
                        <span className={`rounded px-1.5 py-0.5 text-[11px] ${STATE_BADGE[a.state] ?? 'bg-default-100 text-default-600'}`}>
                          {a.state}
                        </span>
                      </td>
                      <td className="px-2 py-1.5 text-default-500">
                        {a.recovery_seconds > 0 ? fmtCountdown(a.recovery_seconds) : '-'}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        ) : (
          <ItemList items={tab === 'platform' ? data?.by_platform ?? [] : data?.by_group ?? []} />
        )}
      </Card>

      {/* 切换率趋势（M13）*/}
      <Card className="mt-3 p-4">
        <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-default-600">
          <Shuffle className="h-4 w-4" />
          {t('ops.switch_rate_title', { defaultValue: '账号切换率（近 1 小时）' })}
        </div>
        {!hasSwitchData ? (
          <div className="flex h-[160px] items-center justify-center text-sm text-default-400">
            {t('ops.no_switch_data', { defaultValue: '暂无切换数据（粘性会话重路由时统计）' })}
          </div>
        ) : (
          <div className="h-[160px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={switchData} margin={{ top: 4, right: 8, left: -18, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--heroui-default-200))" />
                <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Line
                  type="monotone"
                  dataKey="rate"
                  name={t('ops.switch_rate', { defaultValue: '切换率' }) + ' %'}
                  stroke="#a855f7"
                  dot={false}
                  strokeWidth={2}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </Card>
    </div>
  );
}

function AvailCard({
  label,
  value,
  tone,
  hint,
}: {
  label: string;
  value: number;
  tone: 'success' | 'warning' | 'secondary' | 'danger';
  hint?: string;
}) {
  const toneClass: Record<string, string> = {
    success: 'text-success',
    warning: 'text-warning',
    secondary: 'text-secondary',
    danger: 'text-danger',
  };
  return (
    <Card className="p-3">
      <div className="text-xs text-default-500">{label}</div>
      <div className={`mt-0.5 text-xl font-semibold ${toneClass[tone]}`}>{value}</div>
      {hint && <div className="mt-0.5 text-[11px] text-default-400">{hint}</div>}
    </Card>
  );
}
