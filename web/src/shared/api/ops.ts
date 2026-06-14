import { get } from './client';
import type {
  OpsOverviewResp,
  OpsRequestLogResp,
  OpsErrorLogQuery,
  PagedData,
} from '../types';

export const opsApi = {
  // 实时大盘概览（最近窗口 + 趋势）
  overview: (params?: { trend_windows?: number }) =>
    get<OpsOverviewResp>('/api/v1/admin/ops/overview', params),
  // 错误/请求日志列表（分页）
  errorLogs: (params: OpsErrorLogQuery) =>
    get<PagedData<OpsRequestLogResp>>('/api/v1/admin/ops/error-logs', params),
  // 单条日志详情（钻取）
  errorDetail: (id: number) =>
    get<OpsRequestLogResp>(`/api/v1/admin/ops/error-logs/${id}`),
};
