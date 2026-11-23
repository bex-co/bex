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
  className,
  children,
}: {
  variant: "outline" | "destructive" | "default";
  size?: "sm";
  /** The ready button's own size classes, beyond its variant's. */
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <span className="relative inline-flex shrink-0">
      <span
        className={cn(
          buttonVariants({ variant, size }),
          "invisible",
          className,
        )}
      >
        {children}
      </span>
      <SkeletonSpan />
    </span>
  );
}

/**
 * An Ory submit button's exact box at the dashboard's h-9 scale (`style.css`)
 * as a bar: at least 36 px around its 14 px medium label, capped at 488 px and
 * clipping its overflow as Ory's does, so the label wraps as the ready one's
 * does. `style.css` gives a button that follows an Ory input 4 px of block
 * padding instead of 8: `afterInput` is that button, as on the login, sign-up,
 * forgot-password and verification forms, whose hidden csrf input precedes it
 * (w7/093).
 */
export function ReservedOryButton({
  afterInput = false,
  children,
  ...props
}: Omit<React.ComponentProps<"span">, "children"> & {
  afterInput?: boolean;
  children: string;
}) {
  return (
    <span
      className={cn(
        "relative flex min-h-9 max-w-[488px] items-center justify-center overflow-hidden px-4 text-sm leading-5 font-medium",
        afterInput ? "py-1" : "py-2",
      )}
      {...props}
    >
      <ReservedText>{children}</ReservedText>
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
