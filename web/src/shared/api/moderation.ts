import { get, put } from './client';
import type { PagedData } from '../types';

// 审核配置（读：api_keys 仅返回脱敏 hint）。
export interface ModerationConfig {
  enabled: boolean;
  mode: string; // off / observe / pre_block
  keyword_mode: string; // keyword_only / keyword_and_api / api_only
  blocked_keywords: string[];
  api_base_url: string;
  api_model: string;
  api_key_hints: string[];
  timeout_ms: number;
  retry_count: number;
  thresholds: Record<string, number> | null;
  block_status: number;
  block_message: string;
  auto_ban_enabled: boolean;
  ban_threshold: number;
  violation_window_hours: number;
}

// 审核配置写入（api_keys 明文；留空保留原值，不回传 hints）。
export interface ModerationConfigReq {
  enabled: boolean;
  mode: string;
  keyword_mode: string;
  blocked_keywords: string[];
  api_base_url: string;
  api_model: string;
  api_keys: string[];
  timeout_ms: number;
  retry_count: number;
  thresholds: Record<string, number> | null;
  block_status: number;
  block_message: string;
  auto_ban_enabled: boolean;
  ban_threshold: number;
  violation_window_hours: number;
}

export interface ModerationLog {
  id: number;
  request_id: string;
  user_id: number;
  platform: string;
  endpoint: string;
  mode: string;
  flagged: boolean;
  source: string;
  category: string;
  score: number;
  excerpt: string;
  created_at: string;
}

export interface ModerationLogQuery {
  page?: number;
  page_size?: number;
  user_id?: number;
  platform?: string;
  flagged?: number; // 1=命中 0=未命中
}

export const moderationApi = {
  getConfig: () => get<ModerationConfig>('/api/v1/admin/moderation/config'),
  updateConfig: (body: ModerationConfigReq) =>
    put<ModerationConfig>('/api/v1/admin/moderation/config', body),
  logs: (params: ModerationLogQuery) =>
    get<PagedData<ModerationLog>>('/api/v1/admin/moderation/logs', params),
};
