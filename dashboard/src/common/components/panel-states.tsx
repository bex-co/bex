import { Skeleton } from "@/common/components/ui/skeleton";
import { TableHead } from "@/common/components/ui/table";
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
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="text-muted-foreground/50 mb-3 [&_svg]:size-8">{icon}</div>
      <p className="mb-1 font-medium">{title}</p>
      <p className="text-muted-foreground text-sm">{body}</p>
    </div>
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
