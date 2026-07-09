import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";

export interface MetadataRow {
  label: string;
  value: string;
}

export interface MetadataListProps {
  title: string;
  rows: MetadataRow[];
}

/**
 * A titled card of label/value rows — the detail-page "Details" panel shape
 * shared by every resource detail page (databases, Key Value, …): a
 * `dl`/`dt`/`dd` grid, two columns on wider screens, a divider between rows.
 * Presentation only; callers build their own `rows` from their own view type.
 */
export function MetadataList({ title, rows }: MetadataListProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
          {rows.map((row) => (
            <div
              key={row.label}
              className="flex justify-between gap-4 border-b pb-2 last:border-0 sm:last:border-b"
            >
              <dt className="text-sm text-muted-foreground">{row.label}</dt>
              <dd className="truncate text-sm font-medium">{row.value}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}
