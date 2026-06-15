import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { HeartPulse, AlertTriangle, AlertCircle, Info } from 'lucide-react';
import { opsApi } from '../../../shared/api/ops';
import { queryKeys } from '../../../shared/queryKeys';

const REFRESH_MS = 15000;

// 健康等级配色
const GRADE_COLOR: Record<string, { ring: string; text: string }> = {
  healthy: { ring: '#22c55e', text: 'text-success' },
  warning: { ring: '#f59e0b', text: 'text-warning' },
  critical: { ring: '#ef4444', text: 'text-danger' },
};

const LEVEL_ICON: Record<string, React.ReactNode> = {
  critical: <AlertCircle className="h-4 w-4 text-danger" />,
  warning: <AlertTriangle className="h-4 w-4 text-warning" />,
  info: <Info className="h-4 w-4 text-default-400" />,
};

function ScoreRing({ score, color }: { score: number; color: string }) {
  const r = 42;
  const c = 2 * Math.PI * r;
  const offset = c * (1 - score / 100);
  return (
    <div className="relative h-28 w-28 shrink-0">
      <svg className="h-full w-full -rotate-90" viewBox="0 0 100 100">
        <circle cx="50" cy="50" r={r} fill="none" stroke="hsl(var(--heroui-default-200))" strokeWidth="8" />
        <circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          stroke={color}
          strokeWidth="8"
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: 'stroke-dashoffset 0.5s ease' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-3xl font-bold">{score}</span>
        <span className="text-[10px] text-default-400">/ 100</span>
      </div>
    </div>
  );
}

export function OpsHealthCard() {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);

  const query = useQuery({
    queryKey: queryKeys.opsHealth(),
    queryFn: () => opsApi.health(),
    refetchInterval: REFRESH_MS,
    placeholderData: keepPreviousData,
  });
  const data = query.data;
  const grade = data?.grade ?? 'healthy';
  const palette = GRADE_COLOR[grade] ?? { ring: '#22c55e', text: 'text-success' };

  const gradeLabel: Record<string, string> = {
    healthy: t('ops.grade_healthy', { defaultValue: '健康' }),
    warning: t('ops.grade_warning', { defaultValue: '注意' }),
    critical: t('ops.grade_critical', { defaultValue: '严重' }),
  };

  const issues = (data?.diagnostics ?? []).filter((d) => d.level !== 'info');
  const visible = expanded ? data?.diagnostics ?? [] : (data?.diagnostics ?? []).slice(0, 3);

  return (
    <Card className="p-4">
      <div className="flex items-center gap-1.5 text-sm font-medium text-default-600">
        <HeartPulse className="h-4 w-4" />
        {t('ops.health_title', { defaultValue: '健康评分' })}
      </div>
      <div className="mt-3 flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="flex items-center gap-4">
          <ScoreRing score={data?.score ?? 0} color={palette.ring} />
          <div>
            <div className={`text-lg font-semibold ${palette.text}`}>{gradeLabel[grade]}</div>
            <div className="mt-1 text-xs text-default-400">
              {t('ops.health_breakdown', {
                defaultValue: '业务扣 {{b}} · 基础设施扣 {{i}}',
                b: data?.business_sub ?? 0,
                i: data?.infra_sub ?? 0,
              })}
            </div>
            <div className="mt-0.5 text-xs text-default-400">
              {t('ops.health_issues', { defaultValue: '{{n}} 项待关注', n: issues.length })}
            </div>
          </div>
        </div>

        {/* 诊断列表 */}
        <div className="min-w-0 flex-1 space-y-1.5">
          {visible.map((d, i) => (
            <div key={i} className="flex items-start gap-2 rounded-md bg-default-50 px-2.5 py-1.5">
              <div className="mt-0.5">{LEVEL_ICON[d.level] ?? LEVEL_ICON.info}</div>
              <div className="min-w-0 flex-1">
                <div className="text-xs font-medium text-default-700">{d.title}</div>
                {d.detail && <div className="text-[11px] text-default-500">{d.detail}</div>}
                {d.suggestion && (
                  <div className="text-[11px] text-primary-500">{d.suggestion}</div>
                )}
              </div>
            </div>
          ))}
          {(data?.diagnostics ?? []).length > 3 && (
            <button
              className="text-[11px] text-default-400 hover:text-default-600"
              onClick={() => setExpanded((v) => !v)}
            >
              {expanded
                ? t('common.collapse', { defaultValue: '收起' })
                : t('ops.show_all_diagnostics', {
                    defaultValue: '展开全部 {{n}} 项',
                    n: (data?.diagnostics ?? []).length,
                  })}
            </button>
          )}
        </div>
      </div>
    </Card>
  );
}
