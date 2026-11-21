import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Shared empty/error state for a settings-panel body — a centered icon + title +
 * body, rendered INSIDE a `CardContent` (unlike `EmptyState`, which wraps its own
 * Card).
 */
export function PanelCenteredState({
  icon,
  title,
  body,
  action,
}: {
  icon: React.ReactNode;
  title: string;
  /** Text, or text with an inline link (a plan notice's Billing link). */
  body: React.ReactNode;
  /** The way out of this state: an upgrade, a create, a retry. */
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="text-muted-foreground/50 mb-3 [&_svg]:size-8">{icon}</div>
      <p className="mb-1 font-medium">{title}</p>
      <p className="text-muted-foreground text-sm text-pretty max-md:text-base">
        {body}
      </p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

/**
 * PanelCenteredState's loading frame, line for line: icon, title, a body
 * line (two below `lg`, where the body wraps) and the action's button.
 */
export function PanelCenteredStateSkeleton() {
  return (
    <div className="flex flex-col items-center py-12">
      <Skeleton className="mb-3 size-8 rounded-full" />
      <Skeleton className="mb-1 h-6 w-40" />
      <Skeleton className="h-5 w-96 max-w-full" />
      <Skeleton className="h-5 w-56 max-w-full lg:hidden" />
      <Skeleton className="mt-4 h-9 w-24" />
    </div>
  );
}

/**
 * A settings-panel table's header row: its column labels, then the sr-only
 * row-actions head. The ready table and its loading frame
 * (`PanelTableFrame`) share it so the two never drift apart.
 */
export function PanelTableHeader({
  columns,
  actionsLabel,
}: {
  columns: string[];
  /** The sr-only actions head's label; defaults to `common.colActions`. */
  actionsLabel?: string;
}) {
  return (
    <TableHeader>
      <TableRow>
        {columns.map((column) => (
          <TableHead key={column}>{column}</TableHead>
        ))}
        <TableActionsHead label={actionsLabel} />
      </TableRow>
    </TableHeader>
  );
}

/**
 * A settings-panel table's loading frame (w4/686): the ready table's header
 * row, then one placeholder row at a revocable row's height (a text line per
 * column and the 36 px icon-button slot). A panel whose always-present row has
 * another shape passes that row as `children` instead.
 */
export function PanelTableFrame({
  columns,
  actionsLabel,
  children,
}: {
  columns: string[];
  actionsLabel?: string;
  children?: React.ReactNode;
}) {
  return (
    <Table aria-hidden="true" data-testid="panel-table-frame">
      <PanelTableHeader columns={columns} actionsLabel={actionsLabel} />
      <TableBody>
        {children ?? (
          <TableRow data-testid="panel-table-row-placeholder">
            {columns.map((column) => (
              <TableCell key={column}>
                <Skeleton className="h-5 w-16" />
              </TableCell>
            ))}
            <TableCell>
              <Skeleton className="ml-auto size-9" />
            </TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}

/** Shared loading skeleton for a settings-panel table body. */
export function PanelTableSkeleton({ rows = 2 }: { rows?: number }) {
  return (
    <div className="space-y-2">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-6 flex-1" />
        </div>
      ))}
    </div>
  );
}

/**
 * The header cell for a table's trailing row-actions column. Its label exists
 * only for screen readers — sighted users infer the column from the buttons —
 * but it is still user-visible text, so it goes through `t()` like any other.
 */
export function TableActionsHead({ label }: { label?: string }) {
  const { t } = useTranslations();
  return (
    <TableHead className="sr-only text-right">
      {label ?? t("common.colActions")}
    </TableHead>
  );
}
