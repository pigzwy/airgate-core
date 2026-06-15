import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { ShieldAlert } from 'lucide-react';
import { moderationApi, type ModerationConfigReq } from '../../shared/api/moderation';
import { queryKeys } from '../../shared/queryKeys';
import { NativeSwitch } from '../../shared/components/NativeSwitch';
import { useToast } from '../../shared/ui';

const PAGE_SIZE = 50;

type FormState = Omit<ModerationConfigReq, 'api_keys' | 'thresholds'>;

const emptyForm: FormState = {
  enabled: false,
  mode: 'pre_block',
  keyword_mode: 'keyword_and_api',
  blocked_keywords: [],
  api_base_url: '',
  api_model: '',
  timeout_ms: 3000,
  retry_count: 2,
  block_status: 403,
  block_message: '',
  auto_ban_enabled: false,
  ban_threshold: 10,
  violation_window_hours: 720,
};

function fmtTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

export default function ModerationPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // ===== 配置 =====
  const configQuery = useQuery({
    queryKey: queryKeys.moderationConfig(),
    queryFn: () => moderationApi.getConfig(),
  });

  const [form, setForm] = useState<FormState>(emptyForm);
  const [keywordsText, setKeywordsText] = useState('');
  const [apiKeysText, setApiKeysText] = useState('');
  const [thresholdsText, setThresholdsText] = useState('');

  useEffect(() => {
    const c = configQuery.data;
    if (!c) return;
    setForm({
      enabled: c.enabled,
      mode: c.mode || 'pre_block',
      keyword_mode: c.keyword_mode || 'keyword_and_api',
      blocked_keywords: c.blocked_keywords ?? [],
      api_base_url: c.api_base_url,
      api_model: c.api_model,
      timeout_ms: c.timeout_ms,
      retry_count: c.retry_count,
      block_status: c.block_status,
      block_message: c.block_message,
      auto_ban_enabled: c.auto_ban_enabled,
      ban_threshold: c.ban_threshold,
      violation_window_hours: c.violation_window_hours,
    });
    setKeywordsText((c.blocked_keywords ?? []).join('\n'));
    setThresholdsText(c.thresholds ? JSON.stringify(c.thresholds, null, 2) : '');
    setApiKeysText('');
  }, [configQuery.data]);

  const keyHints = configQuery.data?.api_key_hints ?? [];

  const saveMutation = useMutation({
    mutationFn: () => {
      let thresholds: Record<string, number> | null = null;
      if (thresholdsText.trim()) {
        thresholds = JSON.parse(thresholdsText); // 抛错由 onError 捕获
      }
      const body: ModerationConfigReq = {
        ...form,
        blocked_keywords: keywordsText.split('\n').map((s) => s.trim()).filter(Boolean),
        api_keys: apiKeysText.split('\n').map((s) => s.trim()).filter(Boolean),
        thresholds,
      };
      return moderationApi.updateConfig(body);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.moderationConfig() });
      toast('success', t('moderation.saved', { defaultValue: '已保存' }));
    },
    onError: (err: Error) =>
      toast('error', err.message || t('moderation.save_failed', { defaultValue: '保存失败（检查阈值 JSON 格式）' })),
  });

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setForm((f) => ({ ...f, [k]: v }));

  // ===== 日志 =====
  const [page, setPage] = useState(1);
  const [onlyFlagged, setOnlyFlagged] = useState(true);
  const logsQuery = useQuery({
    queryKey: queryKeys.moderationLogs(page, onlyFlagged),
    queryFn: () =>
      moderationApi.logs({ page, page_size: PAGE_SIZE, flagged: onlyFlagged ? 1 : undefined }),
    placeholderData: keepPreviousData,
  });
  const logs = logsQuery.data?.list ?? [];
  const total = logsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const modeOptions = useMemo(
    () => [
      { v: 'off', label: t('moderation.mode_off', { defaultValue: '关闭' }) },
      { v: 'observe', label: t('moderation.mode_observe', { defaultValue: '观察（只记录不拦截）' }) },
      { v: 'pre_block', label: t('moderation.mode_pre_block', { defaultValue: '拦截（命中即拒）' }) },
    ],
    [t],
  );

  const inputCls = 'w-full rounded-md border border-border bg-background px-3 py-2 text-sm outline-none focus:border-primary';

  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center gap-2">
        <ShieldAlert className="h-5 w-5 text-accent" />
        <h1 className="text-lg font-semibold">{t('moderation.title', { defaultValue: '内容审核 / 风控' })}</h1>
      </div>

      {/* 配置 */}
      <Card className="space-y-4 p-5">
        <div className="flex items-center justify-between">
          <NativeSwitch
            isSelected={form.enabled}
            onChange={(v) => set('enabled', v)}
            label={t('moderation.enabled', { defaultValue: '启用内容审核' })}
          />
          <button
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground disabled:opacity-50"
            disabled={saveMutation.isPending}
            onClick={() => saveMutation.mutate()}
          >
            {saveMutation.isPending ? t('common.saving', { defaultValue: '保存中…' }) : t('common.save', { defaultValue: '保存' })}
          </button>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.mode', { defaultValue: '审核模式' })}</span>
            <select className={inputCls} value={form.mode} onChange={(e) => set('mode', e.target.value)}>
              {modeOptions.map((o) => <option key={o.v} value={o.v}>{o.label}</option>)}
            </select>
          </label>
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.keyword_mode', { defaultValue: '检测方式' })}</span>
            <select className={inputCls} value={form.keyword_mode} onChange={(e) => set('keyword_mode', e.target.value)}>
              <option value="keyword_only">{t('moderation.kw_only', { defaultValue: '仅本地词库' })}</option>
              <option value="keyword_and_api">{t('moderation.kw_api', { defaultValue: '词库 + OpenAI API' })}</option>
              <option value="api_only">{t('moderation.api_only', { defaultValue: '仅 OpenAI API' })}</option>
            </select>
          </label>
        </div>

        <label className="block space-y-1">
          <span className="text-sm text-muted">{t('moderation.keywords', { defaultValue: '敏感词（每行一个）' })}</span>
          <textarea className={inputCls} rows={4} value={keywordsText} onChange={(e) => setKeywordsText(e.target.value)} />
        </label>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="space-y-1">
            <span className="text-sm text-muted">API Base URL</span>
            <input className={inputCls} value={form.api_base_url} onChange={(e) => set('api_base_url', e.target.value)} placeholder="https://api.openai.com" />
          </label>
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.api_model', { defaultValue: '审核模型' })}</span>
            <input className={inputCls} value={form.api_model} onChange={(e) => set('api_model', e.target.value)} placeholder="omni-moderation-latest" />
          </label>
        </div>

        <label className="block space-y-1">
          <span className="text-sm text-muted">
            {t('moderation.api_keys', { defaultValue: 'OpenAI API Keys（每行一个，留空保留原有）' })}
            {keyHints.length > 0 && (
              <span className="ml-2 text-xs text-default-400">{t('moderation.current', { defaultValue: '当前' })}: {keyHints.join(', ')}</span>
            )}
          </span>
          <textarea className={inputCls} rows={2} value={apiKeysText} onChange={(e) => setApiKeysText(e.target.value)} placeholder="sk-..." />
        </label>

        <label className="block space-y-1">
          <span className="text-sm text-muted">{t('moderation.thresholds', { defaultValue: '分类阈值（JSON，留空用默认）' })}</span>
          <textarea className={`${inputCls} font-mono`} rows={4} value={thresholdsText} onChange={(e) => setThresholdsText(e.target.value)} placeholder='{"sexual": 0.65}' />
        </label>

        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.block_status', { defaultValue: '拦截状态码' })}</span>
            <input type="number" className={inputCls} value={form.block_status} onChange={(e) => set('block_status', Number(e.target.value))} />
          </label>
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.timeout', { defaultValue: 'API 超时(ms)' })}</span>
            <input type="number" className={inputCls} value={form.timeout_ms} onChange={(e) => set('timeout_ms', Number(e.target.value))} />
          </label>
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.ban_threshold', { defaultValue: '封禁阈值(次)' })}</span>
            <input type="number" className={inputCls} value={form.ban_threshold} onChange={(e) => set('ban_threshold', Number(e.target.value))} />
          </label>
          <label className="space-y-1">
            <span className="text-sm text-muted">{t('moderation.ban_window', { defaultValue: '封禁窗口(小时)' })}</span>
            <input type="number" className={inputCls} value={form.violation_window_hours} onChange={(e) => set('violation_window_hours', Number(e.target.value))} />
          </label>
        </div>

        <label className="block space-y-1">
          <span className="text-sm text-muted">{t('moderation.block_message', { defaultValue: '拦截提示语' })}</span>
          <input className={inputCls} value={form.block_message} onChange={(e) => set('block_message', e.target.value)} placeholder="内容命中风险规则，请调整输入后重试" />
        </label>

        <NativeSwitch
          isSelected={form.auto_ban_enabled}
          onChange={(v) => set('auto_ban_enabled', v)}
          label={t('moderation.auto_ban', { defaultValue: '启用自动封禁（窗口内命中达阈值禁用用户）' })}
        />
      </Card>

      {/* 日志 */}
      <Card className="p-4">
        <div className="mb-3 flex items-center justify-between">
          <div className="text-sm font-medium text-default-600">{t('moderation.logs', { defaultValue: '审核日志' })}</div>
          <NativeSwitch
            isSelected={onlyFlagged}
            onChange={(v) => { setOnlyFlagged(v); setPage(1); }}
            label={t('moderation.only_flagged', { defaultValue: '只看命中' })}
          />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-default-200 text-left text-xs text-default-500">
                <th className="px-2 py-2">{t('moderation.col_time', { defaultValue: '时间' })}</th>
                <th className="px-2 py-2">{t('moderation.col_user', { defaultValue: '用户' })}</th>
                <th className="px-2 py-2">{t('moderation.col_platform', { defaultValue: '平台' })}</th>
                <th className="px-2 py-2">{t('moderation.col_source', { defaultValue: '来源' })}</th>
                <th className="px-2 py-2">{t('moderation.col_category', { defaultValue: '分类' })}</th>
                <th className="px-2 py-2">{t('moderation.col_score', { defaultValue: '分值' })}</th>
                <th className="px-2 py-2">{t('moderation.col_excerpt', { defaultValue: '摘录' })}</th>
              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr><td colSpan={7} className="px-2 py-8 text-center text-default-400">{t('moderation.no_logs', { defaultValue: '暂无审核日志' })}</td></tr>
              ) : logs.map((log) => (
                <tr key={log.id} className="border-b border-default-100">
                  <td className="whitespace-nowrap px-2 py-2 text-default-600">{fmtTime(log.created_at)}</td>
                  <td className="px-2 py-2">{log.user_id || '-'}</td>
                  <td className="px-2 py-2">{log.platform || '-'}</td>
                  <td className="px-2 py-2">{log.source || '-'}</td>
                  <td className="px-2 py-2">{log.category || '-'}</td>
                  <td className="px-2 py-2">{log.flagged ? log.score.toFixed(2) : '-'}</td>
                  <td className="max-w-[280px] truncate px-2 py-2 text-default-500" title={log.excerpt}>{log.excerpt || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="mt-3 flex items-center justify-end gap-2 text-sm">
            <button className="rounded px-2 py-1 disabled:opacity-40" disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>
              {t('common.prev', { defaultValue: '上一页' })}
            </button>
            <span className="text-default-500">{page} / {totalPages}</span>
            <button className="rounded px-2 py-1 disabled:opacity-40" disabled={page >= totalPages} onClick={() => setPage((p) => Math.min(totalPages, p + 1))}>
              {t('common.next', { defaultValue: '下一页' })}
            </button>
          </div>
        )}
      </Card>
    </div>
  );
}
