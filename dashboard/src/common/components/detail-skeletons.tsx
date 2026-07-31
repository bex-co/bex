import { Skeleton } from "@/common/components/ui/skeleton";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";

/**
 * Composable, presentation-only loading skeletons for the resource detail pages
 * (databases, Key Value, blueprints, workspace settings, …). They reuse the real
 * `Card` chrome so a page's loading state reads as its actual multi-panel stack
 * rather than a single undifferentiated rectangle. No text — no i18n.
 */

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
