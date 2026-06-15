import { get, put, post, del } from './client';
import type {
  OpsOverviewResp,
  OpsRequestLogResp,
  OpsErrorLogQuery,
  OpsSystemMetricsResp,
  OpsAnalyticsResp,
  OpsAnalyticsQuery,
  OpsConcurrencyResp,
  OpsSwitchRatePoint,
  OpsSystemLogResp,
  OpsSystemLogQuery,
  OpsLogLevelResp,
  OpsHealthResp,
  OpsAlertRule,
  OpsAlertRuleInput,
  OpsAlertEvent,
  OpsAlertSilence,
  PagedData,
} from '../types';

export const opsApi = {
  // 实时大盘概览（最近窗口 + 趋势）
  overview: (params?: { trend_windows?: number; platform?: string }) =>
    get<OpsOverviewResp>('/api/v1/admin/ops/overview', params),
  // 健康分数 + 诊断
  health: () => get<OpsHealthResp>('/api/v1/admin/ops/health'),
  // 系统资源快照（Goroutines/内存/Redis/后台任务）
  systemMetrics: () =>
    get<OpsSystemMetricsResp>('/api/v1/admin/ops/system-metrics'),
  // 分析（全分位/TTFT/直方图/错误分布/Token/平台维度）
  analytics: (params?: OpsAnalyticsQuery) =>
    get<OpsAnalyticsResp>('/api/v1/admin/ops/analytics', params),
  // 并发统计（账号/平台/分组 + 账号可用性）
  concurrency: () => get<OpsConcurrencyResp>('/api/v1/admin/ops/concurrency'),
  // 切换率趋势
  switchRate: (minutes: number) =>
    get<OpsSwitchRatePoint[]>('/api/v1/admin/ops/switch-rate', { minutes }),
  // 系统日志（分页）
  systemLogs: (params: OpsSystemLogQuery) =>
    get<PagedData<OpsSystemLogResp>>('/api/v1/admin/ops/system-logs', params),
  // 日志级别
  getLogLevel: () => get<OpsLogLevelResp>('/api/v1/admin/ops/log-level'),
  setLogLevel: (level: string) => put<OpsLogLevelResp>('/api/v1/admin/ops/log-level', { level }),
  resetLogLevel: () => post<OpsLogLevelResp>('/api/v1/admin/ops/log-level/reset'),
  // 请求/错误日志列表（分页）
  errorLogs: (params: OpsErrorLogQuery) =>
    get<PagedData<OpsRequestLogResp>>('/api/v1/admin/ops/request-logs', params),
  // 单条日志详情（钻取）
  errorDetail: (id: number) =>
    get<OpsRequestLogResp>(`/api/v1/admin/ops/request-logs/${id}`),
  // 请求链路（同 client_request_id 的所有尝试）
  trace: (clientRequestId: string) =>
    get<OpsRequestLogResp[]>('/api/v1/admin/ops/trace', { client_request_id: clientRequestId }),

  // 告警规则
  listAlertRules: () => get<OpsAlertRule[]>('/api/v1/admin/ops/alert-rules'),
  createAlertRule: (body: OpsAlertRuleInput) =>
    post<OpsAlertRule>('/api/v1/admin/ops/alert-rules', body),
  updateAlertRule: (id: number, body: OpsAlertRuleInput) =>
    put<OpsAlertRule>(`/api/v1/admin/ops/alert-rules/${id}`, body),
  deleteAlertRule: (id: number) => del<{ deleted: boolean }>(`/api/v1/admin/ops/alert-rules/${id}`),
  // 告警事件
  listAlertEvents: (params?: { page?: number; page_size?: number; status?: string; severity?: string }) =>
    get<PagedData<OpsAlertEvent>>('/api/v1/admin/ops/alert-events', params),
  resolveAlertEvent: (id: number) =>
    post<{ resolved: boolean }>(`/api/v1/admin/ops/alert-events/${id}/resolve`),
  // 静音
  listSilences: () => get<OpsAlertSilence[]>('/api/v1/admin/ops/alert-silences'),
  createSilence: (body: { rule_id: number; reason: string; duration_minute: number }) =>
    post<OpsAlertSilence>('/api/v1/admin/ops/alert-silences', body),
  deleteSilence: (id: number) => del<{ deleted: boolean }>(`/api/v1/admin/ops/alert-silences/${id}`),
};
