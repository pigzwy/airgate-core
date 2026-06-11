import type { ComponentType, CSSProperties } from 'react';

export interface PluginOAuthStartResult {
  authorizeURL: string;
  state: string;
}

export interface PluginOAuthExchangeResult {
  accountType: string;
  accountName: string;
  credentials: Record<string, string>;
}

export interface PluginOAuthBatchExchangeResult {
  accountType: string;
  accountName: string;
  credentials: Record<string, string>;
  status: 'ok' | 'failed';
  error?: string;
}

export interface PluginOAuthBridge {
  start: () => Promise<PluginOAuthStartResult>;
  exchange: (callbackURL: string) => Promise<PluginOAuthExchangeResult>;
  batchExchange?: (sessionKeys: string[]) => Promise<PluginOAuthBatchExchangeResult[]>;
  importRefresh?: (
    refreshToken: string,
    clientId?: string,
  ) => Promise<PluginOAuthExchangeResult>;
  batchImportRefresh?: (
    refreshTokens: string[],
    clientId?: string,
  ) => Promise<PluginOAuthBatchExchangeResult[]>;
}

export interface PluginBatchAccountInput {
  name: string;
  type: string;
  credentials: Record<string, string>;
}

export interface PluginBatchImportResult {
  imported: number;
  failed: number;
}

export interface AccountFormProps {
  credentials: Record<string, string>;
  onChange: (credentials: Record<string, string>) => void;
  mode: 'create' | 'edit';
  accountType?: string;
  onAccountTypeChange?: (type: string) => void;
  onSuggestedName?: (name: string) => void;
  onBatchModeChange?: (isBatch: boolean) => void;
  onBatchImport?: (accounts: PluginBatchAccountInput[]) => Promise<PluginBatchImportResult>;
  oauth?: PluginOAuthBridge;
}

export interface PluginRouteDefinition {
  path: string;
  component: ComponentType;
}

export interface PluginMenuItemDefinition {
  path: string;
  title: string;
  icon: string;
}

export interface PluginPlatformIconProps {
  className?: string;
  style?: CSSProperties;
}

export interface AccountSurfaceProps {
  accountId?: string | number;
  accountType?: string;
  context?: Record<string, unknown>;
}

export interface UsageRecordSurfaceProps {
  recordId?: string | number;
  context?: Record<string, unknown>;
}

export interface PluginFrontendModule {
  routes?: PluginRouteDefinition[];
  menuItems?: PluginMenuItemDefinition[];
  accountIdentity?: ComponentType<AccountSurfaceProps>;
  accountCreate?: ComponentType<AccountFormProps>;
  accountEdit?: ComponentType<AccountFormProps>;
  accountUsageWindow?: ComponentType<AccountSurfaceProps>;
  usageModelMeta?: ComponentType<UsageRecordSurfaceProps>;
  usageMetricDetail?: ComponentType<UsageRecordSurfaceProps>;
  usageCostDetail?: ComponentType<UsageRecordSurfaceProps>;
  platformIcon?: ComponentType<PluginPlatformIconProps>;
}
