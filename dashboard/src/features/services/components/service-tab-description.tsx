import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";

/**
 * Pairs a tab's opening line with the detail layout's tab heading above it
 * (12px), apart from the cards below (24px, the layout's section gap).
 */
export const SERVICE_TAB_DESCRIPTION_OFFSET = "-mt-3";

/** A tab's one-line description, under the layout's tab heading. */
export function ServiceTabDescription({ children }: { children: ReactNode }) {
  return (
    <p
      className={cn(
        // 16px on a phone, where it is body text, not a caption.
        "text-muted-foreground max-w-prose text-sm text-pretty max-md:text-base",
        SERVICE_TAB_DESCRIPTION_OFFSET,
      )}
    >
      {children}
    </p>
  );
}
