import { Skeleton } from "@/common/components/ui/skeleton";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";

/**
 * Composable, presentation-only loading skeletons for the resource detail pages
 * (databases, Key Value, blueprints, workspace settings, …). They reuse the real
 * `Card` chrome so a page's loading state reads as its actual multi-panel stack
 * rather than a single undifferentiated rectangle. No text — no i18n.
 */

/**
 * A top-level LIST route's pending state (w9/m69): the page's own content shape —
 * a title + action row over a card grid — inside the persistent shell's content
 * region, instead of the bare centered `RoutePending` spinner. Mounts only when
 * a navigation is slow enough to pass `defaultPendingMs` (a prefetched/cached
 * nav skips it), and matches the real list layout so the skeleton→data swap
 * doesn't jump. Never a full-viewport element — the sidebar/header persist.
 */
export function ListPageSkeleton() {
  return (
    <div className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="w-full space-y-6">
        <div className="flex items-center justify-between gap-2">
          <Skeleton className="h-7 w-44" />
          <Skeleton className="h-9 w-28" />
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <CardSkeleton rows={2} />
          <CardSkeleton rows={2} />
          <CardSkeleton rows={2} />
        </div>
      </div>
    </div>
  );
}

/**
 * `/agents` pending state: a centered composer box over recents rows, not the
 * 3-column service card grid. Other list routes keep `ListPageSkeleton`.
 */
export function AgentsPageSkeleton() {
  return (
    <div
      className="flex-1 overflow-auto p-4 sm:p-6"
      data-testid="agents-page-skeleton"
    >
      <div className="mx-auto w-full max-w-[40rem] space-y-8">
        <Skeleton className="h-40 w-full rounded-xl" />
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full rounded-lg" />
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * A top-level CREATE (`*.new`) route's pending state (w9/m69): a title + a
 * form-shaped card, so the create wizards don't fall back to the bare spinner
 * either. Same content-region wrapper as `ListPageSkeleton`.
 */
export function FormPageSkeleton() {
  return (
    <div className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="mx-auto w-full max-w-2xl space-y-6">
        <Skeleton className="h-7 w-52" />
        <CardSkeleton rows={5} />
      </div>
    </div>
  );
}

/**
 * A titled card placeholder — real Card chrome with a skeleton title line and
 * `rows` body lines of varying width (so it reads as prose, not identical bars).
 * The generic stand-in for a detail-page panel while it loads.
 */
export function CardSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-5 w-40" />
      </CardHeader>
      <CardContent className="space-y-3">
        {Array.from({ length: rows }).map((_, i) => (
          <Skeleton
            key={i}
            className={
              i % 3 === 0
                ? "h-4 w-full"
                : i % 3 === 1
                  ? "h-4 w-5/6"
                  : "h-4 w-2/3"
            }
          />
        ))}
      </CardContent>
    </Card>
  );
}

/**
 * Mirrors `MetadataList` (common/components/metadata-list.tsx): a titled card of
 * `rows` label/value pairs in a two-column grid — the "Details" panel shape shared
 * by every resource detail page.
 */
export function MetadataListSkeleton({ rows = 8 }: { rows?: number }) {
  return (
    <Card>
      <CardHeader>
        <Skeleton className="h-5 w-32" />
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
          {Array.from({ length: rows }).map((_, i) => (
            <div
              key={i}
              className="flex justify-between gap-4 border-b pb-2 last:border-0 sm:last:border-b"
            >
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-16" />
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

/**
 * A stack of `rows` form-field placeholders (a label line above a full-width
 * input). For the settings General card and other edit-in-place forms.
 */
export function FieldRowsSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-4">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="space-y-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-9 w-full" />
        </div>
      ))}
    </div>
  );
}

/**
 * The instance-type picker's card grid, as a skeleton — a 3-column grid of tier
 * cards. Shared by the plan page's initial load and the picker's own loading
 * state so the two agree (instance-type-picker.tsx).
 */
export function PlanPickerGridSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-24 w-full" />
      ))}
    </div>
  );
}
