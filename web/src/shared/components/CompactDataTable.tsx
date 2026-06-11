import { type CSSProperties, type ReactNode } from 'react';

type RowKey = string | number;

export interface CompactDataTableColumn<T> {
  align?: 'start' | 'end';
  key: string;
  render: (row: T, index: number) => ReactNode;
  title: ReactNode;
  width?: CSSProperties['width'];
}

interface CompactDataTableProps<T> {
  ariaLabel: string;
  className?: string;
  columns: CompactDataTableColumn<T>[];
  emptyText: ReactNode;
  minWidth?: CSSProperties['minWidth'];
  rowKey: (row: T, index: number) => RowKey;
  rows: T[];
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

export function CompactDataTable<T>({
  ariaLabel,
  className,
  columns,
  emptyText,
  minWidth,
  rowKey,
  rows,
}: CompactDataTableProps<T>) {
  return (
    <div className={cx('overflow-hidden rounded-[var(--radius)] border border-border bg-surface', className)}>
      <div className="overflow-x-auto" data-slot="wrapper">
        <table
          aria-label={ariaLabel}
          className="w-full border-collapse text-sm"
          data-slot="table"
          style={minWidth ? { minWidth } : undefined}
        >
          <thead data-slot="thead">
            <tr data-slot="tr">
              {columns.map((column, index) => (
                <th
                  data-row-header={index === 0 || undefined}
                  data-slot="th"
                  id={column.key}
                  key={column.key}
                  scope="col"
                  className={cx(
                    'border-b border-separator bg-default px-3 py-2 text-xs font-semibold text-muted',
                    column.align === 'end' ? 'text-right' : undefined,
                  )}
                  style={column.width ? { width: column.width } : undefined}
                >
                  <span
                    className={cx(
                      'flex items-center',
                      column.align === 'end' ? 'justify-end text-right' : 'justify-start text-left',
                    )}
                  >
                    {column.title}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody data-slot="tbody">
            {rows.length === 0 ? (
              <tr data-key="empty" data-slot="tr">
                <td colSpan={columns.length} data-slot="td">
                  <div className="flex min-h-28 items-center justify-center px-3 py-8 text-sm text-muted">{emptyText}</div>
                </td>
              </tr>
            ) : rows.map((row, rowIndex) => {
                const key = rowKey(row, rowIndex);

                return (
                  <tr data-key={String(key)} data-slot="tr" key={key}>
                    {columns.map((column) => (
                      <td
                        data-slot="td"
                        key={column.key}
                        className={cx(
                          'border-b border-separator px-3 py-2 align-middle text-foreground',
                          column.align === 'end' ? 'text-right' : undefined,
                        )}
                      >
                        <div
                          className={cx(
                            'flex min-h-8 items-center',
                            column.align === 'end' ? 'justify-end text-right' : 'justify-start text-left',
                          )}
                        >
                          {column.render(row, rowIndex)}
                        </div>
                      </td>
                    ))}
                  </tr>
                );
              })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
