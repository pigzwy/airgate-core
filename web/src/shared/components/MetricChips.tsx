import { Chip } from '@heroui/react';

type MetricChipColor = 'default' | 'warning' | 'success' | 'accent' | 'danger';

const DOLLAR_TEXT_CLASS: Record<MetricChipColor, string> = {
  accent: 'text-accent',
  danger: 'text-danger',
  default: 'text-foreground',
  success: 'text-success',
  warning: 'text-warning',
};

export type MetricChipItem = {
  amount?: number;
  color: MetricChipColor;
  decimals?: number;
  dollarTone?: MetricChipColor;
  highlightDollar?: boolean;
  label: string;
  mutedWhenZero?: boolean;
  value?: string;
};

function formatMoneyAmount(value: number, decimals = 4) {
  return (Number.isFinite(value) ? value : 0).toFixed(decimals);
}

function formatMetricTitleValue(item: MetricChipItem) {
  if (item.amount != null) return `$${formatMoneyAmount(item.amount, item.decimals)}`;
  return item.value ?? '';
}

function MetricChip({ amount, color, decimals, dollarTone, highlightDollar, label, mutedWhenZero, value }: MetricChipItem) {
  const amountText = amount == null ? null : formatMoneyAmount(amount, decimals);
  const isMutedZero = mutedWhenZero && amount === 0;
  const dollarColor = dollarTone ?? (highlightDollar ? 'warning' : undefined);

  return (
    <Chip color={isMutedZero ? 'default' : color} size="sm" variant="soft">
      <span>{label}</span>
      <span>
        {amountText == null ? (
          value === '∞' ? <span>{value}</span> : value
        ) : (
          <>
            <span className={dollarColor ? DOLLAR_TEXT_CLASS[dollarColor] : undefined}>$</span>
            <span>{amountText}</span>
          </>
        )}
      </span>
    </Chip>
  );
}

export function MetricChips({
  className,
  items,
}: {
  className?: string;
  items: MetricChipItem[];
}) {
  const title = items
    .map((item) => `${item.label} ${formatMetricTitleValue(item)}`)
    .join(' / ');

  return (
    <div className={className} title={title}>
      {items.map((item, idx) => (
        <MetricChip key={`${idx}-${item.label}`} {...item} />
      ))}
    </div>
  );
}
