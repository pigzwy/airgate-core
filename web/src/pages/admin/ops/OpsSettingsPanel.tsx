import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { Settings2, Check } from 'lucide-react';
import { settingsApi } from '../../../shared/api/settings';
import { queryKeys } from '../../../shared/queryKeys';
import { useOpsSettings } from '../../../shared/hooks/useOpsSettings';

const inputCls = 'w-full rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm';

export function OpsSettingsPanel() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { settings } = useOpsSettings();

  // 本地编辑态（从已加载设置初始化）
  const [form, setForm] = useState({
    errorRateWarn: settings.errorRateWarn,
    errorRateCrit: settings.errorRateCrit,
    p95Warn: settings.p95Warn,
    p95Crit: settings.p95Crit,
    goroutineWarn: settings.goroutineWarn,
    retentionLogDays: settings.retentionLogDays,
    retentionStatDays: settings.retentionStatDays,
    retentionSyslogDays: settings.retentionSyslogDays,
    reportEnabled: settings.reportEnabled,
    reportEmail: settings.reportEmail,
    reportHour: settings.reportHour,
  });
  const [saved, setSaved] = useState(false);

  // 设置加载完成后同步一次（避免覆盖用户正在编辑的内容：仅在首次/外部刷新时同步）
  useEffect(() => {
    setForm({
      errorRateWarn: settings.errorRateWarn,
      errorRateCrit: settings.errorRateCrit,
      p95Warn: settings.p95Warn,
      p95Crit: settings.p95Crit,
      goroutineWarn: settings.goroutineWarn,
      retentionLogDays: settings.retentionLogDays,
      retentionStatDays: settings.retentionStatDays,
      retentionSyslogDays: settings.retentionSyslogDays,
      reportEnabled: settings.reportEnabled,
      reportEmail: settings.reportEmail,
      reportHour: settings.reportHour,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    settings.errorRateWarn,
    settings.p95Warn,
    settings.retentionLogDays,
    settings.reportEnabled,
    settings.reportEmail,
    settings.reportHour,
  ]);

  const save = useMutation({
    mutationFn: () =>
      settingsApi.update({
        settings: [
          { key: 'ops_threshold_error_rate_warn', value: String(form.errorRateWarn), group: 'ops' },
          { key: 'ops_threshold_error_rate_crit', value: String(form.errorRateCrit), group: 'ops' },
          { key: 'ops_threshold_p95_warn', value: String(form.p95Warn), group: 'ops' },
          { key: 'ops_threshold_p95_crit', value: String(form.p95Crit), group: 'ops' },
          { key: 'ops_threshold_goroutine_warn', value: String(form.goroutineWarn), group: 'ops' },
          { key: 'ops_retention_log_days', value: String(form.retentionLogDays), group: 'ops' },
          { key: 'ops_retention_stat_days', value: String(form.retentionStatDays), group: 'ops' },
          { key: 'ops_retention_syslog_days', value: String(form.retentionSyslogDays), group: 'ops' },
          { key: 'ops_report_enabled', value: form.reportEnabled ? 'true' : 'false', group: 'ops' },
          { key: 'ops_report_email', value: form.reportEmail, group: 'ops' },
          { key: 'ops_report_hour', value: String(form.reportHour), group: 'ops' },
        ],
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.opsSettings() });
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    },
  });

  const set = (patch: Partial<typeof form>) => setForm((f) => ({ ...f, ...patch }));
  const numInput = (key: keyof typeof form, step = '1') => (
    <input
      type="number"
      step={step}
      className={inputCls}
      value={form[key] as number}
      onChange={(e) => set({ [key]: Number(e.target.value) } as Partial<typeof form>)}
    />
  );

  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-sm font-medium text-default-600">
          <Settings2 className="h-4 w-4" />
          {t('ops.settings_title', { defaultValue: '运维设置' })}
        </div>
        <button
          className="flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground disabled:opacity-50"
          disabled={save.isPending}
          onClick={() => save.mutate()}
        >
          {saved ? <Check className="h-3.5 w-3.5" /> : null}
          {saved ? t('common.saved', { defaultValue: '已保存' }) : t('common.save', { defaultValue: '保存' })}
        </button>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {/* 阈值染色 */}
        <div>
          <div className="mb-2 text-xs font-medium text-default-500">
            {t('ops.settings_thresholds', { defaultValue: '阈值染色' })}
          </div>
          <div className="space-y-2">
            <Field label={t('ops.th_error_warn', { defaultValue: '错误率告警(比例)' })}>{numInput('errorRateWarn', '0.01')}</Field>
            <Field label={t('ops.th_error_crit', { defaultValue: '错误率严重(比例)' })}>{numInput('errorRateCrit', '0.01')}</Field>
            <Field label={t('ops.th_p95_warn', { defaultValue: 'P95 告警(ms)' })}>{numInput('p95Warn')}</Field>
            <Field label={t('ops.th_p95_crit', { defaultValue: 'P95 严重(ms)' })}>{numInput('p95Crit')}</Field>
            <Field label={t('ops.th_goroutine', { defaultValue: 'Goroutine 告警' })}>{numInput('goroutineWarn')}</Field>
          </div>
        </div>

        {/* 保留策略 */}
        <div>
          <div className="mb-2 text-xs font-medium text-default-500">
            {t('ops.settings_retention', { defaultValue: '保留策略（天）' })}
          </div>
          <div className="space-y-2">
            <Field label={t('ops.ret_log', { defaultValue: '请求日志' })}>{numInput('retentionLogDays')}</Field>
            <Field label={t('ops.ret_stat', { defaultValue: '聚合窗口' })}>{numInput('retentionStatDays')}</Field>
            <Field label={t('ops.ret_syslog', { defaultValue: '系统日志' })}>{numInput('retentionSyslogDays')}</Field>
          </div>
        </div>

        {/* 邮件日报 */}
        <div>
          <div className="mb-2 text-xs font-medium text-default-500">
            {t('ops.settings_report', { defaultValue: '邮件日报' })}
          </div>
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.reportEnabled}
                onChange={(e) => set({ reportEnabled: e.target.checked })}
              />
              {t('ops.report_enabled', { defaultValue: '启用每日报告' })}
            </label>
            <Field label={t('ops.report_email', { defaultValue: '收件邮箱(逗号分隔)' })}>
              <input
                className={inputCls}
                value={form.reportEmail}
                onChange={(e) => set({ reportEmail: e.target.value })}
                placeholder="ops@example.com"
              />
            </Field>
            <Field label={t('ops.report_hour', { defaultValue: '发送时刻(0-23)' })}>{numInput('reportHour')}</Field>
          </div>
        </div>
      </div>
      <div className="mt-3 text-[11px] text-default-400">
        {t('ops.settings_note', {
          defaultValue: '阈值用于大盘卡片染色；保留策略与邮件日报由后台任务实时读取生效。',
        })}
      </div>
    </Card>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="mb-0.5 block text-[11px] text-default-400">{label}</label>
      {children}
    </div>
  );
}
