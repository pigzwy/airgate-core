import type {
  ComponentPropsWithoutRef,
  CSSProperties,
  HTMLAttributes,
  ReactNode,
  TdHTMLAttributes,
  ThHTMLAttributes,
} from 'react';

type NativeTableProps = ComponentPropsWithoutRef<'table'>;

const DEFAULT_ROW_HEADER_COLUMN_IDS = new Set(['action', 'email', 'id', 'name', 'user_id']);

interface CommonTableProps {
  ariaLabel: string;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  contentProps?: Omit<NativeTableProps, 'aria-label' | 'children' | 'className' | 'style'>;
  contentStyle?: CSSProperties;
  footer?: ReactNode;
  minWidth?: number | string;
  scrollClassName?: string;
  scrollOverlay?: ReactNode;
}

type CommonTableColumnProps = Omit<ThHTMLAttributes<HTMLTableCellElement>, 'id'> & {
  id?: string;
  isRowHeader?: boolean;
};
type CommonTableRowProps = Omit<HTMLAttributes<HTMLTableRowElement>, 'id'> & {
  id?: string | number;
};

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

function CommonTableRoot({
  ariaLabel,
  children,
  className,
  contentClassName,
  contentProps,
  contentStyle,
  footer,
  minWidth,
  scrollClassName,
  scrollOverlay,
}: CommonTableProps) {
  const resolvedContentStyle = minWidth == null
    ? contentStyle
    : {
        minWidth,
        ...contentStyle,
      };

  return (
    <div className={cx('overflow-hidden rounded-[var(--radius)] border border-border bg-surface', className)}>
      <div className={cx('relative overflow-x-auto', scrollClassName)} data-slot="wrapper">
        {scrollOverlay}
        <table
          {...contentProps}
          aria-label={ariaLabel}
          className={cx('w-full border-collapse text-sm', contentClassName)}
          data-slot="table"
          style={resolvedContentStyle}
        >
          {children}
        </table>
      </div>
      {footer ? (
        <div className="border-t border-separator bg-surface px-3 py-2" data-slot="table-footer">
          {footer}
        </div>
      ) : null}
    </div>
  );
}

function CommonTableHeader({ children, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <thead data-slot="thead" {...props}>
      <tr data-slot="tr">{children}</tr>
    </thead>
  );
}

function CommonTableBody({ children, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <tbody data-slot="tbody" {...props}>
      {children}
    </tbody>
  );
}

function CommonTableColumn({ id, isRowHeader, children, className, ...props }: CommonTableColumnProps) {
  const shouldMarkRowHeader =
    isRowHeader ?? (typeof id === 'string' && DEFAULT_ROW_HEADER_COLUMN_IDS.has(id));

  return (
    <th
      {...props}
      data-row-header={shouldMarkRowHeader || undefined}
      data-slot="th"
      id={id}
      scope="col"
      className={cx('border-b border-separator bg-default px-3 py-2 text-xs font-semibold text-muted', className)}
    >
      {children}
    </th>
  );
}

function CommonTableRow({ id, children, className, ...props }: CommonTableRowProps) {
  return (
    <tr {...props} className={cx('transition-colors hover:bg-default/60', className)} data-key={id == null ? undefined : String(id)} data-slot="tr">
      {children}
    </tr>
  );
}

function CommonTableCell({ children, className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td {...props} className={cx('border-b border-separator px-3 py-2 align-middle text-foreground', className)} data-slot="td">
      {children}
    </td>
  );
}

export const CommonTable = Object.assign(CommonTableRoot, {
  Body: CommonTableBody,
  Cell: CommonTableCell,
  Column: CommonTableColumn,
  Header: CommonTableHeader,
  Row: CommonTableRow,
});
