import { useQuery } from '@tanstack/react-query';
import { settingsApi } from '../api/settings';
import { queryKeys } from '../queryKeys';

// 运维设置（M15）默认值。与后端 app/ops/config.go 的默认保持一致。
export const OPS_SETTINGS_DEFAULTS = {
  errorRateWarn: 0.02,
  errorRateCrit: 0.08,
  p95Warn: 3000,
  p95Crit: 8000,
  goroutineWarn: 2000,
  retentionLogDays: 7,
  retentionStatDays: 30,
  retentionSyslogDays: 7,
  reportEnabled: false,
  reportEmail: '',
  reportHour: 9,
};

export interface OpsSettings extends Omit<typeof OPS_SETTINGS_DEFAULTS, 'reportEnabled' | 'reportEmail'> {
  reportEnabled: boolean;
  reportEmail: string;
  raw: Record<string, string>;
}

function num(raw: Record<string, string>, key: string, def: number): number {
  const v = Number(raw[key]);
  return Number.isFinite(v) && raw[key] !== '' && raw[key] !== undefined ? v : def;
}

export function useOpsSettings() {
  const query = useQuery({
    queryKey: queryKeys.opsSettings(),
    queryFn: () => settingsApi.listGroup('ops'),
    staleTime: 60000,
  });

  const raw: Record<string, string> = {};
  for (const s of query.data ?? []) raw[s.key] = s.value;

  const settings: OpsSettings = {
    errorRateWarn: num(raw, 'ops_threshold_error_rate_warn', OPS_SETTINGS_DEFAULTS.errorRateWarn),
    errorRateCrit: num(raw, 'ops_threshold_error_rate_crit', OPS_SETTINGS_DEFAULTS.errorRateCrit),
    p95Warn: num(raw, 'ops_threshold_p95_warn', OPS_SETTINGS_DEFAULTS.p95Warn),
    p95Crit: num(raw, 'ops_threshold_p95_crit', OPS_SETTINGS_DEFAULTS.p95Crit),
    goroutineWarn: num(raw, 'ops_threshold_goroutine_warn', OPS_SETTINGS_DEFAULTS.goroutineWarn),
    retentionLogDays: num(raw, 'ops_retention_log_days', OPS_SETTINGS_DEFAULTS.retentionLogDays),
    retentionStatDays: num(raw, 'ops_retention_stat_days', OPS_SETTINGS_DEFAULTS.retentionStatDays),
    retentionSyslogDays: num(raw, 'ops_retention_syslog_days', OPS_SETTINGS_DEFAULTS.retentionSyslogDays),
    reportEnabled: raw['ops_report_enabled'] === 'true',
    reportEmail: raw['ops_report_email'] ?? '',
    reportHour: num(raw, 'ops_report_hour', OPS_SETTINGS_DEFAULTS.reportHour),
    raw,
  };

  return { settings, isLoading: query.isLoading };
}
