import { SkeletonSpan } from "@/common/components/ui/skeleton";
import { Badge } from "@/common/components/ui/badge";
import { buttonVariants } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";

/**
 * Static copy's footprint without its ink: the ready page's own translated
 * string laid out invisibly (so it wraps exactly as it will at this width and
 * in this language) under a bar. The parent carries the ready element's type
 * classes plus `relative`. Every bar here is a `<span>`, so either may sit in a
 * `<p>` without breaking hydration (w4/686).
 */
export function ReservedText({ children }: { children: React.ReactNode }) {
  return (
    <>
      <span className="invisible">{children}</span>
      <SkeletonSpan />
    </>
  );
}

/** A ready button's exact box — same variant, size and label — as a bar. */
export function ReservedButton({
  variant,
  size,
  children,
}: {
  variant: "outline" | "destructive" | "default";
  size?: "sm";
  children: React.ReactNode;
}) {
  return (
    <span className="relative inline-flex shrink-0">
      <span className={cn(buttonVariants({ variant, size }), "invisible")}>
        {children}
      </span>
      <SkeletonSpan />
    </span>
  );
}

/** A ready Badge's exact box — same variant and text — as a bar. `region`
 *  names the box for the pending-frame tests (`data-skeleton-region`). */
export function ReservedBadge({
  variant,
  region,
  children,
}: {
  variant: "secondary" | "outline";
  region?: string;
  children: React.ReactNode;
}) {
  return (
    <span
      className="relative inline-flex shrink-0"
      data-skeleton-region={region}
    >
      <Badge variant={variant} className="invisible">
        {children}
      </Badge>
      <SkeletonSpan />
    </span>
  );
}
