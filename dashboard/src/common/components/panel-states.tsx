import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * Shared empty/error state for a settings-panel body — a centered icon + title +
 * body, rendered INSIDE a `CardContent` (unlike `EmptyState`, which wraps its own
 * Card). Used by the API Keys and Team panels so their empty/forbidden/error
 * states stay visually identical.
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

/** Shared two-row loading skeleton for a settings-panel table body. */
export function PanelTableSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1].map((i) => (
        <div key={i} className="flex items-center gap-4">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-6 flex-1" />
        </div>
      ))}
    </div>
  );
}
