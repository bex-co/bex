import type { ReactNode } from "react";

import {
  formatRelativeAge,
  formatRelativeUntil,
} from "@/features/services/lib/format";

interface RelativeTimeProps {
  /** RFC3339/ISO instant to render relative to "now". */
  value: string | null | undefined;
  /** Rendered when `value` is missing or unparseable. Defaults to an em dash. */
  fallback?: ReactNode;
  className?: string;
  /** `title` attribute — usually the exact timestamp behind the relative text. */
  title?: string;
  /**
   * Element to render. `time` (the default) also carries the machine-readable
   * instant in `dateTime`; pass `span` when the call site already sits inside a
   * `<time>` (nesting `<time>` is invalid HTML, and `suppressHydrationWarning`
   * only reaches one level, so the inner element needs its own guard anyway).
   */
  as?: "time" | "span";
}

function RelativeTime({
  value,
  format,
  fallback = "—",
  className,
  title,
  as = "time",
}: RelativeTimeProps & { format: (iso: string) => string }) {
  // No usable instant: render the caller's fallback bare, so `dateTime` never
  // carries an empty or unparseable value.
  if (!value || Number.isNaN(Date.parse(value))) return <>{fallback}</>;

  // formatRelativeAge/Until measure elapsed time against a fresh Date.now(),
  // evaluated once during the SSR pass and again during client hydration. Any
  // bucket boundary crossed in the gap between the two (60s/60min/24h/30d/12mo)
  // makes the same element render different text on the server and the client —
  // React error #418 (w6/m102; the same class as w6/030's formatDateTime
  // timezone divergence, a different mechanism under the same error code). The
  // machine-readable instant in `dateTime` stays exact; only the human
  // rendering is allowed to differ.
  const text = format(value);
  return as === "span" ? (
    <span className={className} title={title} suppressHydrationWarning>
      {text}
    </span>
  ) : (
    <time
      dateTime={value}
      className={className}
      title={title}
      suppressHydrationWarning
    >
      {text}
    </time>
  );
}

/**
 * Compact age since a past instant ("now", "4m", "3h", "5d", "2mo"), with the
 * hydration guard baked in — the guarded counterpart of `formatRelativeAge`.
 * Every UI call site should use this instead of calling the formatter directly.
 */
export function RelativeAge(props: RelativeTimeProps) {
  return <RelativeTime {...props} format={formatRelativeAge} />;
}

/**
 * Compact time until a future instant ("in 5m", "in 3h", "in 2d"), with the
 * hydration guard baked in — the guarded counterpart of `formatRelativeUntil`.
 */
export function RelativeUntil(props: RelativeTimeProps) {
  return <RelativeTime {...props} format={formatRelativeUntil} />;
}
