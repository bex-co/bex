import type { ReactNode } from "react";

import { cn } from "@/common/lib/utils/utils";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { formatDateTime, formatDateLong } from "@/common/lib/format";

interface LocalTimeProps {
  /** RFC3339/ISO instant to render in the viewer's local timezone. */
  value: string | null | undefined;
  /** Rendered when `value` is missing or unparseable. Defaults to an em dash. */
  fallback?: ReactNode;
  className?: string;
  /** `title` attribute — usually the exact instant behind the text. */
  title?: string;
  /**
   * Element to render. `time` (the default) also carries the machine-readable
   * instant in `dateTime`; pass `span` when the call site already sits inside a
   * `<time>` (nesting `<time>` is invalid HTML).
   */
  as?: "time" | "span";
}

/**
 * The reserved-width placeholder shown until hydration completes. `formatDateTime`
 * / `formatDateLong` read the *runtime's* timezone (UTC in the SSR container, the
 * viewer's in the browser), so the server can't know the correct local text —
 * rendering it during SSR bakes a wrong, UTC-clock value into the markup that
 * `suppressHydrationWarning` then freezes on screen (w6/030 → w6/m107). Deferring
 * to a post-hydration render is the only way to show the viewer's own time without
 * ever flashing a wrong one.
 */
function LocalTime({
  value,
  format,
  fallback = "—",
  className,
  title,
  as = "time",
  placeholderClassName,
}: LocalTimeProps & {
  format: (iso: string) => string | null;
  placeholderClassName: string;
}) {
  const hydrated = useIsHydrated();
  const parsed = value && !Number.isNaN(Date.parse(value)) ? value : null;
  if (!parsed) return <>{fallback}</>;

  const content = !hydrated ? (
    <Skeleton
      className={cn(
        "inline-block h-4 align-[-0.2em]",
        placeholderClassName,
      )}
    />
  ) : (
    (format(parsed) ?? fallback)
  );

  return as === "span" ? (
    <span className={className} title={title}>
      {content}
    </span>
  ) : (
    <time dateTime={parsed} className={className} title={title}>
      {content}
    </time>
  );
}

/**
 * Absolute date + time in the viewer's local timezone ("July 16, 2026 at 12:57
 * AM"), with the hydration guard baked in — the client-safe counterpart of
 * `formatDateTime`. Every UI call site should use this (or `useLocalDateTime`
 * for string/attribute contexts) instead of calling the formatter directly in a
 * render body.
 */
export function LocalDateTime(props: LocalTimeProps) {
  return (
    <LocalTime {...props} format={formatDateTime} placeholderClassName="w-40" />
  );
}

/**
 * Absolute date in the viewer's local timezone ("July 16, 2026"), with the
 * hydration guard baked in — the client-safe counterpart of `formatDateLong`.
 * Narrower divergence than `LocalDateTime` (only instants within the local/UTC
 * gap around a calendar-day boundary land on different days) but the same root
 * cause, so it takes the same treatment.
 */
export function LocalDate(props: LocalTimeProps) {
  return (
    <LocalTime {...props} format={formatDateLong} placeholderClassName="w-28" />
  );
}
