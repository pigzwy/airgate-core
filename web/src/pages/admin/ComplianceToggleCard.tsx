import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card } from '@heroui/react';
import { ShieldCheck } from 'lucide-react';
import { complianceApi } from '../../shared/api/compliance';
import { queryKeys } from '../../shared/queryKeys';
import { NativeSwitch } from '../../shared/components/NativeSwitch';
import { useToast } from '../../shared/ui';

// ComplianceToggleCard 管理员合规确认门开关（系统设置 → 安全）。
//
// 自包含：自带查询与开关 mutation，不参与设置页的批量保存。开启后，管理员的下一次
// 管理端请求会返回 423，由全局 AdminComplianceGate 弹出确认框。
export function ComplianceToggleCard() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const { data: status, isLoading } = useQuery({
    queryKey: [...queryKeys.compliance(), 'settings'],
    queryFn: () => complianceApi.status(),
  });

  const mutation = useMutation({
    mutationFn: (enabled: boolean) => complianceApi.setEnabled(enabled),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.compliance() });
      toast('success', t('compliance.saved', { defaultValue: '已保存' }));
    },
    onError: (err: Error) => toast('error', err.message),
  });

  return (
    <Card className="space-y-4 p-5">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 text-accent">
          <ShieldCheck className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-foreground">
            {t('compliance.gate_title', { defaultValue: '管理员合规确认门' })}
          </h3>
          <p className="mt-1 text-xs leading-5 text-muted">
            {t('compliance.gate_desc', {
              defaultValue:
                '开启后，管理员首次进入后台须逐字确认合规承诺才能使用管理功能；合规文档版本更新后需重新确认。',
            })}
          </p>
        </div>
      </div>

      <NativeSwitch
        isSelected={!!status?.enabled}
        isDisabled={isLoading || mutation.isPending}
        onChange={(v) => mutation.mutate(v)}
        label={t('compliance.gate_toggle', { defaultValue: '启用合规确认门' })}
      />

      {status && (
        <div className="text-xs text-muted">
          {t('compliance.version', { defaultValue: '版本' })} {status.version}
          {' · '}
          {status.acknowledged
            ? t('compliance.you_acknowledged', { defaultValue: '你已确认当前版本' })
            : t('compliance.you_not_acknowledged', { defaultValue: '你尚未确认当前版本' })}
        </div>
      )}
    </Card>
  );
}
