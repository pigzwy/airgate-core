import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { ScrollText, RotateCcw } from 'lucide-react';
import { opsApi } from '../../../shared/api/ops';
import { queryKeys } from '../../../shared/queryKeys';

const PAGE_SIZE = 50;
const LEVELS = ['debug', 'info', 'warn', 'error'] as const;

const LEVEL_BADGE: Record<string, string> = {
  debug: 'bg-default-100 text-default-600',
  info: 'bg-primary-100 text-primary-700',
  warn: 'bg-warning-100 text-warning-700',
  error: 'bg-danger-100 text-danger-700',
};

function fmtTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export function OpsSystemLogPanel() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [level, setLevel] = useState('');
  const [keyword, setKeyword] = useState('');
  const [keywordInput, setKeywordInput] = useState('');

  // 日志级别
  const levelQuery = useQuery({
    queryKey: queryKeys.opsLogLevel(),
    queryFn: () => opsApi.getLogLevel(),
    refetchInterval: 30000,
  });
  const setLevelMut = useMutation({
    mutationFn: (lvl: string) => opsApi.setLogLevel(lvl),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.opsLogLevel() }),
  });
  const resetLevelMut = useMutation({
    mutationFn: () => opsApi.resetLogLevel(),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.opsLogLevel() }),
  });

  // 日志列表
  const logsQuery = useQuery({
    queryKey: queryKeys.opsSystemLogs(page, level, keyword),
    queryFn: () =>
      opsApi.systemLogs({
        page,
        page_size: PAGE_SIZE,
        level: level || undefined,
        keyword: keyword || undefined,
      }),
    placeholderData: keepPreviousData,
  });
  const logs = logsQuery.data?.list ?? [];
  const total = logsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const runtimeLevel = levelQuery.data?.level;
  const defaultLevel = levelQuery.data?.default;
  const dropped = levelQuery.data?.dropped ?? 0;

  return (
    <Card className="p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-1.5 text-sm font-medium text-default-600">
          <ScrollText className="h-4 w-4" />
          {t('ops.system_logs', { defaultValue: '系统日志' })}
          {dropped > 0 && (
            <span className="text-[11px] font-normal text-warning-500">
              ({t('ops.logs_dropped', { defaultValue: '丢弃 {{n}}', n: dropped })})
            </span>
          )}
        </div>

        {/* 运行时日志级别 */}
        <div className="flex items-center gap-2 text-xs">
          <span className="text-default-400">{t('ops.runtime_level', { defaultValue: '运行级别' })}</span>
          <div className="flex overflow-hidden rounded-md border border-default-200">
            {LEVELS.map((lvl) => (
              <button
                key={lvl}
                disabled={setLevelMut.isPending}
                className={`px-2 py-1 uppercase ${
                  runtimeLevel === lvl
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-default-50 text-default-600 hover:bg-default-100'
                }`}
                onClick={() => setLevelMut.mutate(lvl)}
              >
                {lvl}
              </button>
            ))}
          </div>
          {defaultLevel && runtimeLevel !== defaultLevel && (
            <button
              className="flex items-center gap-1 rounded-md border border-default-200 px-2 py-1 text-default-500 hover:bg-default-100"
              onClick={() => resetLevelMut.mutate()}
              title={t('ops.reset_level', { defaultValue: '恢复默认 {{l}}', l: defaultLevel })}
            >
              <RotateCcw className="h-3 w-3" />
              {t('common.reset', { defaultValue: '重置' })}
            </button>
          )}
        </div>
      </div>

      {/* 筛选 */}
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <select
          aria-label={t('ops.level', { defaultValue: '级别' })}
          className="rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm"
          value={level}
          onChange={(e) => {
            setLevel(e.target.value);
            setPage(1);
          }}
        >
          <option value="">{t('ops.all_levels', { defaultValue: '全部级别' })}</option>
          {LEVELS.map((lvl) => (
            <option key={lvl} value={lvl}>
              {lvl.toUpperCase()}
            </option>
          ))}
        </select>
        <input
          className="flex-1 min-w-[160px] rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm"
          placeholder={t('ops.search_message', { defaultValue: '搜索日志内容…' })}
          value={keywordInput}
          onChange={(e) => setKeywordInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              setKeyword(keywordInput.trim());
              setPage(1);
            }
          }}
        />
        <button
          className="rounded-md border border-default-200 bg-default-50 px-3 py-1.5 text-sm hover:bg-default-100"
          onClick={() => {
            setKeyword(keywordInput.trim());
            setPage(1);
          }}
        >
          {t('common.search', { defaultValue: '搜索' })}
        </button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-default-200 text-left text-xs text-default-500">
              <th className="px-2 py-2">{t('ops.col_time', { defaultValue: '时间' })}</th>
              <th className="px-2 py-2">{t('ops.level', { defaultValue: '级别' })}</th>
              <th className="px-2 py-2">{t('ops.col_component', { defaultValue: '组件' })}</th>
              <th className="px-2 py-2">{t('ops.col_message', { defaultValue: '消息' })}</th>
            </tr>
          </thead>
          <tbody>
            {logs.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-2 py-8 text-center text-default-400">
                  {t('ops.no_logs', { defaultValue: '暂无记录' })}
                </td>
              </tr>
            ) : (
              logs.map((log) => (
                <tr key={log.id} className="border-b border-default-100 align-top hover:bg-default-50">
                  <td className="whitespace-nowrap px-2 py-1.5 text-default-600">{fmtTime(log.created_at)}</td>
                  <td className="px-2 py-1.5">
                    <span className={`rounded px-1.5 py-0.5 text-[11px] uppercase ${LEVEL_BADGE[log.level] ?? 'bg-default-100 text-default-600'}`}>
                      {log.level}
                    </span>
                  </td>
                  <td className="px-2 py-1.5 text-default-500">{log.component || '-'}</td>
                  <td className="px-2 py-1.5">
                    <div className="text-default-700">{log.message}</div>
                    {log.attrs && log.attrs !== '{}' && (
                      <div className="mt-0.5 truncate text-[11px] text-default-400" title={log.attrs}>
                        {log.attrs}
                      </div>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

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
  );
}
