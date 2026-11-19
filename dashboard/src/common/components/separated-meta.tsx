import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";

/**
 * A meta line's items with a muted "·" between neighbours (the Deploys list's
 * and the Events feed's detail lines). Every item carries its separator in
 * front; the line is shifted left by one separator's width and clipped there,
 * so the separator of whichever item starts a line — the first, or one that
 * wrapped — falls outside and is never drawn. A wrapped line never starts
 * with a dangling "·". Clipped across only, so a link's taller tap area and
 * focus ring still show. Items keep to one line each.
 */
export function SeparatedMeta({
  className,
  children,
}: {
  /** The line's place in its parent, and its type and colour (inherited). */
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("min-w-0 overflow-x-clip", className)}>
      <div className="-ml-4 flex flex-wrap items-center gap-y-0.5 [&>*]:whitespace-nowrap [&>*]:before:inline-block [&>*]:before:w-4 [&>*]:before:text-center [&>*]:before:text-muted-foreground [&>*]:before:content-['·']">
        {children}
      </div>
    </div>
  );
}
