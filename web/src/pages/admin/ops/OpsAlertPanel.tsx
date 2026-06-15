import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, useOverlayState } from '@heroui/react';
import { BellRing, Plus, Trash2, BellOff } from 'lucide-react';
import { opsApi } from '../../../shared/api/ops';
import { CommonModal } from '../../../shared/components/CommonModal';
import { queryKeys } from '../../../shared/queryKeys';
import type { OpsAlertRule, OpsAlertRuleInput } from '../../../shared/types';

// 可监控的指标（与后端 validMetrics 对齐）
const METRICS = [
  { key: 'error_rate', label: '错误率', unit: '比例(0-1)' },
  { key: 'p95_latency_ms', label: 'P95 延迟', unit: 'ms' },
  { key: 'p99_latency_ms', label: 'P99 延迟', unit: 'ms' },
  { key: 'ttft_p95_ms', label: 'TTFT P95', unit: 'ms' },
  { key: 'rps', label: 'RPS', unit: '次/秒' },
  { key: 'total_requests', label: '区间请求数', unit: '次' },
  { key: 'db_pool_usage', label: 'DB 连接使用率', unit: '比例(0-1)' },
  { key: 'redis_timeouts', label: 'Redis 超时', unit: '次' },
  { key: 'goroutines', label: 'Goroutines', unit: '个' },
  { key: 'health_score', label: '健康分数', unit: '0-100' },
];
const OPERATORS = [
  { key: 'gt', label: '>' },
  { key: 'gte', label: '≥' },
  { key: 'lt', label: '<' },
  { key: 'lte', label: '≤' },
  { key: 'eq', label: '=' },
];
const SEVERITIES = ['info', 'warning', 'critical'] as const;

const SEVERITY_BADGE: Record<string, string> = {
  info: 'bg-default-100 text-default-600',
  warning: 'bg-warning-100 text-warning-700',
  critical: 'bg-danger-100 text-danger-700',
};

const emptyRule: OpsAlertRuleInput = {
  name: '',
  metric: 'error_rate',
  operator: 'gt',
  threshold: 0.05,
  window_seconds: 300,
  severity: 'warning',
  enabled: true,
  cooldown_seconds: 600,
  notify_email: '',
  platform: '',
};

function fmtTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

function metricLabel(key: string): string {
  return METRICS.find((m) => m.key === key)?.label ?? key;
}
function operatorLabel(key: string): string {
  return OPERATORS.find((o) => o.key === key)?.label ?? key;
}

export function OpsAlertPanel() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [tab, setTab] = useState<'events' | 'rules' | 'silences'>('events');
  const [form, setForm] = useState<OpsAlertRuleInput>(emptyRule);
  const [editId, setEditId] = useState<number | null>(null);

  const ruleModal = useOverlayState();

  // ===== 数据 =====
  const rulesQuery = useQuery({
    queryKey: queryKeys.opsAlertRules(),
    queryFn: () => opsApi.listAlertRules(),
  });
  const eventsQuery = useQuery({
    queryKey: queryKeys.opsAlertEvents(),
    queryFn: () => opsApi.listAlertEvents({ page: 1, page_size: 50 }),
    refetchInterval: 15000,
    placeholderData: keepPreviousData,
  });
  const silencesQuery = useQuery({
    queryKey: queryKeys.opsAlertSilences(),
    queryFn: () => opsApi.listSilences(),
  });

  const rules = rulesQuery.data ?? [];
  const events = eventsQuery.data?.list ?? [];
  const silences = silencesQuery.data ?? [];
  const firingCount = events.filter((e) => e.status === 'firing').length;

  // ===== 变更 =====
  const invalidateRules = () => qc.invalidateQueries({ queryKey: queryKeys.opsAlertRules() });
  const saveRule = useMutation({
    mutationFn: (body: OpsAlertRuleInput) =>
      editId ? opsApi.updateAlertRule(editId, body) : opsApi.createAlertRule(body),
    onSuccess: () => {
      invalidateRules();
      ruleModal.close();
    },
  });
  const deleteRule = useMutation({
    mutationFn: (id: number) => opsApi.deleteAlertRule(id),
    onSuccess: invalidateRules,
  });
  const toggleRule = useMutation({
    mutationFn: (rule: OpsAlertRule) =>
      opsApi.updateAlertRule(rule.id, { ...ruleToInput(rule), enabled: !rule.enabled }),
    onSuccess: invalidateRules,
  });
  const resolveEvent = useMutation({
    mutationFn: (id: number) => opsApi.resolveAlertEvent(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.opsAlertEvents() }),
  });
  const deleteSilence = useMutation({
    mutationFn: (id: number) => opsApi.deleteSilence(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.opsAlertSilences() }),
  });
  const createSilence = useMutation({
    mutationFn: (body: { rule_id: number; reason: string; duration_minute: number }) =>
      opsApi.createSilence(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.opsAlertSilences() }),
  });

  const openCreate = () => {
    setEditId(null);
    setForm(emptyRule);
    ruleModal.open();
  };
  const openEdit = (rule: OpsAlertRule) => {
    setEditId(rule.id);
    setForm(ruleToInput(rule));
    ruleModal.open();
  };

  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-default-600">
        <BellRing className="h-4 w-4" />
        {t('ops.alert_title', { defaultValue: '告警' })}
        {firingCount > 0 && (
          <span className="rounded bg-danger-100 px-1.5 py-0.5 text-[11px] text-danger-700">
            {t('ops.alert_firing', { defaultValue: '{{n}} 触发中', n: firingCount })}
          </span>
        )}
      </div>

      <Card className="p-4">
        {/* Tab */}
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="flex overflow-hidden rounded-md border border-default-200">
            {([
              ['events', t('ops.alert_events', { defaultValue: '事件' })],
              ['rules', t('ops.alert_rules', { defaultValue: '规则' })],
              ['silences', t('ops.alert_silences', { defaultValue: '静音' })],
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
          {tab === 'rules' && (
            <button
              className="flex items-center gap-1 rounded-md bg-primary px-2.5 py-1 text-xs text-primary-foreground"
              onClick={openCreate}
            >
              <Plus className="h-3.5 w-3.5" />
              {t('ops.add_rule', { defaultValue: '新建规则' })}
            </button>
          )}
        </div>

        {tab === 'events' && (
          <EventsTable
            events={events}
            onResolve={(id) => resolveEvent.mutate(id)}
            resolving={resolveEvent.isPending}
          />
        )}
        {tab === 'rules' && (
          <RulesTable
            rules={rules}
            onEdit={openEdit}
            onDelete={(id) => deleteRule.mutate(id)}
            onToggle={(r) => toggleRule.mutate(r)}
            onSilence={(ruleId) =>
              createSilence.mutate({ rule_id: ruleId, reason: '手动静音', duration_minute: 60 })
            }
          />
        )}
        {tab === 'silences' && (
          <SilencesTable silences={silences} rules={rules} onDelete={(id) => deleteSilence.mutate(id)} />
        )}
      </Card>

      {/* 规则编辑弹窗 */}
      <CommonModal
        state={ruleModal}
        title={editId ? t('ops.edit_rule', { defaultValue: '编辑规则' }) : t('ops.add_rule', { defaultValue: '新建规则' })}
        dialogStyle={{ maxWidth: '520px', width: 'min(100%, calc(100vw - 2rem))' }}
      >
        <RuleForm form={form} setForm={setForm} />
        <div className="mt-4 flex justify-end gap-2">
          <button
            className="rounded-md border border-default-200 px-3 py-1.5 text-sm"
            onClick={() => ruleModal.close()}
          >
            {t('common.cancel', { defaultValue: '取消' })}
          </button>
          <button
            className="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground disabled:opacity-50"
            disabled={!form.name || saveRule.isPending}
            onClick={() => saveRule.mutate(form)}
          >
            {t('common.save', { defaultValue: '保存' })}
          </button>
        </div>
      </CommonModal>
    </div>
  );
}

function ruleToInput(r: OpsAlertRule): OpsAlertRuleInput {
  return {
    name: r.name,
    metric: r.metric,
    operator: r.operator,
    threshold: r.threshold,
    window_seconds: r.window_seconds,
    severity: r.severity,
    enabled: r.enabled,
    cooldown_seconds: r.cooldown_seconds,
    notify_email: r.notify_email,
    platform: r.platform,
  };
}

function EventsTable({
  events,
  onResolve,
  resolving,
}: {
  events: import('../../../shared/types').OpsAlertEvent[];
  onResolve: (id: number) => void;
  resolving: boolean;
}) {
  const { t } = useTranslation();
  if (events.length === 0) {
    return <div className="py-6 text-center text-sm text-default-400">{t('ops.no_events', { defaultValue: '暂无告警事件' })}</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-default-200 text-left text-xs text-default-500">
            <th className="px-2 py-1.5">{t('ops.col_time', { defaultValue: '时间' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_rule', { defaultValue: '规则' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_severity', { defaultValue: '严重度' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_status', { defaultValue: '状态' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_message', { defaultValue: '消息' })}</th>
            <th className="px-2 py-1.5"></th>
          </tr>
        </thead>
        <tbody>
          {events.map((e) => (
            <tr key={e.id} className="border-b border-default-100">
              <td className="whitespace-nowrap px-2 py-1.5 text-default-600">{fmtTime(e.created_at)}</td>
              <td className="px-2 py-1.5">{e.rule_name}</td>
              <td className="px-2 py-1.5">
                <span className={`rounded px-1.5 py-0.5 text-[11px] ${SEVERITY_BADGE[e.severity] ?? ''}`}>
                  {e.severity}
                </span>
              </td>
              <td className="px-2 py-1.5">
                {e.status === 'firing' ? (
                  <span className="text-danger">{t('ops.status_firing', { defaultValue: '触发中' })}</span>
                ) : (
                  <span className="text-success">{t('ops.status_resolved', { defaultValue: '已恢复' })}</span>
                )}
              </td>
              <td className="max-w-[320px] truncate px-2 py-1.5 text-default-500" title={e.message}>
                {e.message}
              </td>
              <td className="px-2 py-1.5 text-right">
                {e.status === 'firing' && (
                  <button
                    className="text-xs text-primary disabled:opacity-50"
                    disabled={resolving}
                    onClick={() => onResolve(e.id)}
                  >
                    {t('ops.resolve', { defaultValue: '解决' })}
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RulesTable({
  rules,
  onEdit,
  onDelete,
  onToggle,
  onSilence,
}: {
  rules: OpsAlertRule[];
  onEdit: (r: OpsAlertRule) => void;
  onDelete: (id: number) => void;
  onToggle: (r: OpsAlertRule) => void;
  onSilence: (ruleId: number) => void;
}) {
  const { t } = useTranslation();
  if (rules.length === 0) {
    return <div className="py-6 text-center text-sm text-default-400">{t('ops.no_rules', { defaultValue: '暂无规则，点击右上角新建' })}</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-default-200 text-left text-xs text-default-500">
            <th className="px-2 py-1.5">{t('ops.col_rule', { defaultValue: '规则' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_condition', { defaultValue: '条件' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_severity', { defaultValue: '严重度' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_enabled', { defaultValue: '启用' })}</th>
            <th className="px-2 py-1.5 text-right">{t('common.actions', { defaultValue: '操作' })}</th>
          </tr>
        </thead>
        <tbody>
          {rules.map((r) => (
            <tr key={r.id} className="border-b border-default-100">
              <td className="px-2 py-1.5">{r.name}</td>
              <td className="px-2 py-1.5 text-default-500">
                {metricLabel(r.metric)} {operatorLabel(r.operator)} {r.threshold}
              </td>
              <td className="px-2 py-1.5">
                <span className={`rounded px-1.5 py-0.5 text-[11px] ${SEVERITY_BADGE[r.severity] ?? ''}`}>
                  {r.severity}
                </span>
              </td>
              <td className="px-2 py-1.5">
                <button
                  className={`rounded px-1.5 py-0.5 text-[11px] ${
                    r.enabled ? 'bg-success-100 text-success-700' : 'bg-default-100 text-default-500'
                  }`}
                  onClick={() => onToggle(r)}
                >
                  {r.enabled ? t('common.on', { defaultValue: '开' }) : t('common.off', { defaultValue: '关' })}
                </button>
              </td>
              <td className="px-2 py-1.5 text-right">
                <div className="flex items-center justify-end gap-2">
                  <button className="text-xs text-primary" onClick={() => onEdit(r)}>
                    {t('common.edit', { defaultValue: '编辑' })}
                  </button>
                  <button
                    className="text-default-400 hover:text-warning"
                    title={t('ops.silence_1h', { defaultValue: '静音 1 小时' })}
                    onClick={() => onSilence(r.id)}
                  >
                    <BellOff className="h-3.5 w-3.5" />
                  </button>
                  <button className="text-default-400 hover:text-danger" onClick={() => onDelete(r.id)}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SilencesTable({
  silences,
  rules,
  onDelete,
}: {
  silences: import('../../../shared/types').OpsAlertSilence[];
  rules: OpsAlertRule[];
  onDelete: (id: number) => void;
}) {
  const { t } = useTranslation();
  if (silences.length === 0) {
    return <div className="py-6 text-center text-sm text-default-400">{t('ops.no_silences', { defaultValue: '暂无静音' })}</div>;
  }
  const ruleName = (id: number) =>
    id === 0 ? t('ops.all_rules', { defaultValue: '全部规则' }) : rules.find((r) => r.id === id)?.name ?? `#${id}`;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-default-200 text-left text-xs text-default-500">
            <th className="px-2 py-1.5">{t('ops.col_rule', { defaultValue: '规则' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_reason', { defaultValue: '原因' })}</th>
            <th className="px-2 py-1.5">{t('ops.col_until', { defaultValue: '截止' })}</th>
            <th className="px-2 py-1.5 text-right">{t('common.actions', { defaultValue: '操作' })}</th>
          </tr>
        </thead>
        <tbody>
          {silences.map((s) => (
            <tr key={s.id} className="border-b border-default-100">
              <td className="px-2 py-1.5">{ruleName(s.rule_id)}</td>
              <td className="px-2 py-1.5 text-default-500">{s.reason || '-'}</td>
              <td className="px-2 py-1.5 text-default-500">{fmtTime(s.until)}</td>
              <td className="px-2 py-1.5 text-right">
                <button className="text-default-400 hover:text-danger" onClick={() => onDelete(s.id)}>
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RuleForm({ form, setForm }: { form: OpsAlertRuleInput; setForm: (f: OpsAlertRuleInput) => void }) {
  const { t } = useTranslation();
  const set = (patch: Partial<OpsAlertRuleInput>) => setForm({ ...form, ...patch });
  const inputCls = 'w-full rounded-md border border-default-200 bg-default-50 px-2 py-1.5 text-sm';
  return (
    <div className="space-y-3">
      <div>
        <label className="mb-1 block text-xs text-default-500">{t('ops.field_name', { defaultValue: '规则名称' })}</label>
        <input className={inputCls} value={form.name} onChange={(e) => set({ name: e.target.value })} />
      </div>
      <div className="grid grid-cols-3 gap-2">
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_metric', { defaultValue: '指标' })}</label>
          <select className={inputCls} value={form.metric} onChange={(e) => set({ metric: e.target.value })}>
            {METRICS.map((m) => (
              <option key={m.key} value={m.key}>
                {m.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_operator', { defaultValue: '运算符' })}</label>
          <select className={inputCls} value={form.operator} onChange={(e) => set({ operator: e.target.value })}>
            {OPERATORS.map((o) => (
              <option key={o.key} value={o.key}>
                {o.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_threshold', { defaultValue: '阈值' })}</label>
          <input
            type="number"
            step="any"
            className={inputCls}
            value={form.threshold}
            onChange={(e) => set({ threshold: Number(e.target.value) })}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_window', { defaultValue: '窗口(秒)' })}</label>
          <input
            type="number"
            className={inputCls}
            value={form.window_seconds}
            onChange={(e) => set({ window_seconds: Number(e.target.value) })}
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_cooldown', { defaultValue: '冷却(秒)' })}</label>
          <input
            type="number"
            className={inputCls}
            value={form.cooldown_seconds}
            onChange={(e) => set({ cooldown_seconds: Number(e.target.value) })}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_severity', { defaultValue: '严重度' })}</label>
          <select className={inputCls} value={form.severity} onChange={(e) => set({ severity: e.target.value })}>
            {SEVERITIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs text-default-500">{t('ops.field_platform', { defaultValue: '平台(可选)' })}</label>
          <input className={inputCls} value={form.platform} onChange={(e) => set({ platform: e.target.value })} />
        </div>
      </div>
      <div>
        <label className="mb-1 block text-xs text-default-500">{t('ops.field_notify_email', { defaultValue: '通知邮箱(逗号分隔)' })}</label>
        <input
          className={inputCls}
          value={form.notify_email}
          onChange={(e) => set({ notify_email: e.target.value })}
          placeholder="ops@example.com"
        />
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={form.enabled} onChange={(e) => set({ enabled: e.target.checked })} />
        {t('ops.field_enabled', { defaultValue: '启用规则' })}
      </label>
    </div>
  );
}
