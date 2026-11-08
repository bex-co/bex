import { AtSign } from "lucide-react";
import { Skeleton } from "@/common/components/ui/skeleton";
import { buttonVariants } from "@/common/components/ui/button";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";

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
 * `/agents` pending state. Its default prompt workspace is composer-only;
 * explicit Archived/All/phase URLs render the page heading plus history list.
 * Other list routes keep `ListPageSkeleton`.
 */
export function AgentsPageSkeleton({
  mode = "composer",
}: {
  mode?: "composer" | "list";
}) {
  return (
    <div
      aria-hidden="true"
      className="min-h-0 min-w-0 flex-1 overflow-auto"
      data-route-skeleton="agents-list"
      data-testid="agents-page-skeleton"
    >
      <div className="mx-auto w-full max-w-5xl space-y-8 p-4 sm:p-6">
        {mode === "composer" ? (
          <div
            className="mx-auto w-full max-w-[40rem] space-y-3"
            data-skeleton-region="composer"
          >
            <ComposerBoxSkeleton />
            <Skeleton
              className="h-4 w-64 max-w-full"
              data-skeleton-region="composer-hint"
            />
          </div>
        ) : (
          <>
            <div className="mx-auto w-full max-w-4xl">
              <Skeleton
                className="h-7 w-40"
                data-skeleton-region="page-header"
              />
            </div>
            <div
              className="mx-auto w-full max-w-4xl space-y-3"
              data-skeleton-region="session-list"
            >
              <Skeleton className="h-5 w-24" />
              <div className="space-y-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-14 w-full rounded-lg" />
                ))}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

/**
 * The new-session prompt box while `/agents` loads: the ready composer's own
 * chrome (bordered box, 112px editor) over a toolbar with the same flex-wrap,
 * gap and padding, holding one placeholder per control at that control's size.
 * The two text controls (the repository mention and Start) carry their real
 * label invisibly, so the toolbar wraps at exactly the ready composer's widths,
 * in every locale and whatever width the page gives the composer. Wrapping by
 * viewport breakpoints drifted 36px on desktop, where the composer is capped
 * at 40rem inside the app shell (w9 browser check, 2026-11-06).
 */
function ComposerBoxSkeleton() {
  const { t } = useTranslations();
  // Ghost sizing carries no background, so the Skeleton's own pulse shows.
  const textControl = buttonVariants({ variant: "ghost", size: "sm" });
  return (
    <div
      className="rounded-xl border shadow-sm"
      data-skeleton-region="composer-box"
    >
      <div className="min-h-28" />
      <div
        className="flex flex-wrap items-center gap-1 px-1.5 pb-1.5"
        data-skeleton-region="composer-toolbar"
      >
        {/* Agent select, attach, Secrets, Skills, spend limit. */}
        <Skeleton className="h-8 w-[7.5rem]" />
        <Skeleton className="size-8" />
        <Skeleton className="size-8" />
        <Skeleton className="size-8" />
        <Skeleton className="h-8 w-28" />
        <Skeleton
          className={`${textControl} h-8 max-w-56 gap-1.5 px-2 text-xs font-normal text-transparent`}
        >
          <AtSign className="size-3.5" />
          <span className="truncate">{t("agentSessions.mentionButton")}</span>
        </Skeleton>
        <Skeleton className={`${textControl} ml-auto h-8 text-transparent`}>
          {t("agentSessions.submit")}
        </Skeleton>
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
      <div className="mx-auto w-full max-w-4xl space-y-6">
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
        <FieldRowSkeleton key={i} />
      ))}
    </div>
  );
}

export function FieldRowSkeleton({
  hint = false,
  hintLines = 1,
}: {
  hint?: boolean;
  hintLines?: number;
}) {
  return (
    <div className="space-y-2">
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-9 w-full" />
      {hint ? (
        <div className="space-y-1">
          {Array.from({ length: hintLines }, (_, index) => (
            <Skeleton
              key={index}
              className={index === 0 ? "h-4 w-4/5" : "h-4 w-2/3"}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
