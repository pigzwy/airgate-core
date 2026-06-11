import type { CSSProperties, ReactNode } from 'react';
import { Switch } from '@heroui/react';

interface NativeSwitchProps {
  ariaLabel?: string;
  children?: ReactNode;
  className?: string;
  contentClassName?: string;
  contentStyle?: CSSProperties;
  isDisabled?: boolean;
  isSelected: boolean;
  label?: ReactNode;
  name?: string;
  onChange: (selected: boolean) => void;
}

export function NativeSwitch({
  ariaLabel,
  children,
  className,
  contentClassName,
  contentStyle,
  isDisabled = false,
  isSelected,
  label,
  name,
  onChange,
}: NativeSwitchProps) {
  const content = label ?? children;
  const rootClassName = ['inline-flex items-center gap-3', className].filter(Boolean).join(' ');
  const labelClassName = ['text-sm text-foreground', contentClassName].filter(Boolean).join(' ');

  return (
    <Switch
      aria-label={ariaLabel}
      className={rootClassName}
      isDisabled={isDisabled}
      isSelected={isSelected}
      name={name}
      size="sm"
      onChange={onChange}
    >
      <Switch.Control>
        <Switch.Thumb />
      </Switch.Control>
      {content ? (
        <Switch.Content className={labelClassName} style={contentStyle}>
          {content}
        </Switch.Content>
      ) : null}
    </Switch>
  );
}
