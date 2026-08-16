import type { ReactNode } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";

export interface MetadataRow {
  label: string;
  value: ReactNode;
}

export interface MetadataListProps {
  title: string;
  rows: MetadataRow[];
  /**
   * Optional editable content rendered inside the card, above the read-only
   * `dl` — the datastore detail pages lead their General card with the
   * edit-in-place Name row (w5/m55), mirroring Render's General section (an
   * editable Name atop the read-only facts).
   */
  lead?: ReactNode;
}

/**
 * A titled card of label/value rows — the detail-page "Details" panel shape
 * shared by every resource detail page (databases, Key Value, …): a
 * `dl`/`dt`/`dd` grid, two columns on wider screens, a divider between rows.
 * Presentation only; callers build their own `rows` from their own view type.
 * An optional `lead` slot holds edit-in-place fields above the read-only grid.
 */
export function MetadataList({ title, rows, lead }: MetadataListProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {lead != null && <div className="mb-4 border-b pb-4">{lead}</div>}
        <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
          {rows.map((row) => (
            <div
              key={row.label}
              className="flex justify-between gap-4 border-b pb-2 last:border-0 sm:last:border-b"
            >
              <dt className="text-sm text-muted-foreground">{row.label}</dt>
              <dd
                className="min-w-0 text-sm font-medium"
                suppressHydrationWarning
              >
                {row.value}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}
