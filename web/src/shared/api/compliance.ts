import { get, post, put } from './client';

// 一条管理员合规确认记录。
export interface ComplianceAck {
  version: string;
  admin_user_id: number;
  language: string;
  accepted_at: string;
}

// 合规确认状态（GET /admin/compliance 返回）。
export interface ComplianceStatus {
  required: boolean; // 是否仍需确认
  enabled: boolean; // 合规门是否开启
  acknowledged: boolean; // 是否已确认当前版本
  version: string;
  language: string;
  phrase: string; // 当前语言的逐字确认短语
  document_zh: string; // 中文合规文档全文（markdown）
  document_en: string; // 英文合规文档全文（markdown）
  ack?: ComplianceAck;
}

export const complianceApi = {
  // 查询当前管理员的合规确认状态（含文档与短语）。
  status: (language?: string) =>
    get<ComplianceStatus>('/api/v1/admin/compliance', language ? { language } : undefined),
  // 提交确认（逐字短语）。
  accept: (phrase: string, language: string) =>
    post<ComplianceStatus>('/api/v1/admin/compliance/accept', { phrase, language }),
  // 开启/关闭合规门。
  setEnabled: (enabled: boolean) =>
    put<ComplianceStatus>('/api/v1/admin/compliance/enable', { enabled }),
};
