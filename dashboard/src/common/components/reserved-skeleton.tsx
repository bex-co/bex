import { Skeleton } from "@/common/components/ui/skeleton";
import { buttonVariants } from "@/common/components/ui/button";
import { cn } from "@/common/lib/utils/utils";

/**
 * Static copy's footprint without its ink: the ready page's own translated
 * string laid out invisibly (so it wraps exactly as it will at this width and
 * in this language) under a bar. The parent carries the ready element's type
 * classes plus `relative`.
 */
export function ReservedText({ children }: { children: React.ReactNode }) {
  return (
    <>
      <span className="invisible">{children}</span>
      <Skeleton className="absolute inset-0" />
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
      <Skeleton className="absolute inset-0" />
    </span>
  );
}
