import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, useOverlayState } from '@heroui/react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { CommonModal } from './CommonModal';
import { complianceApi } from '../api/compliance';
import { ApiError, setComplianceRequiredHandler } from '../api/client';
import { queryKeys } from '../queryKeys';

// 合规文档 markdown 渲染样式（精简版）。
const MD_COMPONENTS: Components = {
  h1: (p) => <h1 className="mb-2 mt-1 text-base font-semibold text-foreground" {...p} />,
  h2: (p) => <h2 className="mb-1.5 mt-3 text-sm font-semibold text-foreground" {...p} />,
  p: (p) => <p className="my-2 text-sm leading-6 text-foreground" {...p} />,
  ul: (p) => <ul className="my-2 list-disc space-y-1 pl-5 text-sm text-foreground" {...p} />,
  ol: (p) => <ol className="my-2 list-decimal space-y-1 pl-5 text-sm text-foreground" {...p} />,
  li: (p) => <li className="leading-6" {...p} />,
  strong: (p) => <strong className="font-semibold text-foreground" {...p} />,
  blockquote: (p) => <blockquote className="my-2 border-l-2 border-border pl-3 text-muted" {...p} />,
  code: (p) => <code className="rounded bg-default px-1 py-0.5 font-mono text-xs" {...p} />,
  pre: (p) => <pre className="my-2 overflow-auto rounded bg-default p-2 font-mono text-xs" {...p} />,
};

// AdminComplianceGate 管理员合规确认门弹窗。
//
// 工作方式：任一管理端请求返回 423（需确认合规）时，client.ts 触发本组件注册的
// 全局回调 → 弹出确认框。门关闭（默认）时后端不会返 423，本组件永不弹出。
// 仅管理员会话会触发；普通用户/API Key 会话即便挂载也不会弹。
//
// 确认门为合规法律要求，弹出后不可手动关闭，必须逐字输入确认短语后提交才放行。
export function AdminComplianceGate() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [lang, setLang] = useState<'zh' | 'en'>(i18n.language === 'en' ? 'en' : 'zh');
  const [phrase, setPhrase] = useState('');

  // 注册 / 注销全局 423 回调
  useEffect(() => {
    setComplianceRequiredHandler(() => setOpen(true));
    return () => setComplianceRequiredHandler(null);
  }, []);

  const statusQuery = useQuery({
    queryKey: [...queryKeys.compliance(), lang],
    queryFn: () => complianceApi.status(lang),
    enabled: open,
  });
  const status = statusQuery.data;

  const acceptMutation = useMutation({
    mutationFn: () => complianceApi.accept(phrase.trim(), lang),
    onSuccess: () => {
      setOpen(false);
      setPhrase('');
      // 重新拉取所有因 423 失败的查询
      void queryClient.invalidateQueries();
    },
  });

  // 合规门：不允许手动关闭，仅在确认成功后程序化置 open=false 关闭。
  const modalState = useOverlayState({
    isOpen: open,
    onOpenChange: () => {},
  });

  if (!open) return null;

  const doc = lang === 'en' ? status?.document_en : status?.document_zh;
  const expected = status?.phrase ?? '';
  const canSubmit = !!expected && phrase.trim() === expected && !acceptMutation.isPending;
  const acceptError =
    acceptMutation.error instanceof ApiError ? acceptMutation.error.message : '';

  return (
    <CommonModal
      state={modalState}
      showCloseTrigger={false}
      surface={false}
      title={t('compliance.title', { defaultValue: '管理员合规确认' })}
      description={t('compliance.subtitle', {
        defaultValue: '使用管理功能前，请阅读并确认以下合规承诺。',
      })}
      dialogStyle={{ maxWidth: '760px', width: 'min(100%, calc(100vw - 2rem))' }}
      footer={
        <div className="flex w-full items-center justify-between gap-3">
          <span className="text-xs text-muted">
            {status?.version ? `${t('compliance.version', { defaultValue: '版本' })} ${status.version}` : ''}
          </span>
          <Button variant="primary" isDisabled={!canSubmit} onPress={() => acceptMutation.mutate()}>
            {acceptMutation.isPending
              ? t('compliance.submitting', { defaultValue: '提交中…' })
              : t('compliance.accept', { defaultValue: '我已阅读并同意' })}
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        {/* 语言切换 */}
        <div className="flex items-center justify-end gap-1 text-xs">
          <button
            type="button"
            className={`rounded px-2 py-1 ${lang === 'zh' ? 'bg-default text-foreground' : 'text-muted'}`}
            onClick={() => setLang('zh')}
          >
            中文
          </button>
          <button
            type="button"
            className={`rounded px-2 py-1 ${lang === 'en' ? 'bg-default text-foreground' : 'text-muted'}`}
            onClick={() => setLang('en')}
          >
            EN
          </button>
        </div>

        {/* 合规文档全文 */}
        <div className="max-h-[46vh] overflow-auto rounded-md border border-border bg-surface p-4">
          {statusQuery.isLoading ? (
            <span className="text-sm text-muted">{t('common.loading', { defaultValue: '加载中…' })}</span>
          ) : doc ? (
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={MD_COMPONENTS}>
              {doc}
            </ReactMarkdown>
          ) : (
            '-'
          )}
        </div>

        {/* 逐字确认短语 */}
        <div className="space-y-1.5">
          <label className="text-sm text-muted">
            {t('compliance.phrase_label', { defaultValue: '请逐字输入以下确认短语：' })}
          </label>
          <div className="select-all rounded bg-default px-3 py-2 font-mono text-sm text-foreground">
            {expected || '…'}
          </div>
          <input
            type="text"
            value={phrase}
            onChange={(e) => setPhrase(e.target.value)}
            placeholder={t('compliance.phrase_placeholder', { defaultValue: '在此输入上方短语' })}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm outline-none focus:border-primary"
          />
          {acceptError && <div className="text-xs text-danger">{acceptError}</div>}
        </div>
      </div>
    </CommonModal>
  );
}
