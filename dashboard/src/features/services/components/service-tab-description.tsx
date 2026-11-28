import type { ReactNode } from "react";

/**
 * Pairs a tab's opening line with the detail layout's tab heading above it
 * (12px), apart from the cards below (24px, the layout's section gap).
 */
export const SERVICE_TAB_DESCRIPTION_OFFSET = "-mt-3";

/** The description's type, measure and offset (16px on a phone, where it is
 *  body text, not a caption). A loading frame lays the same copy out in them,
 *  so it wraps where the ready line does (w3/124). */
export const SERVICE_TAB_DESCRIPTION_CLASS = `text-muted-foreground max-w-prose text-sm text-pretty max-md:text-base ${SERVICE_TAB_DESCRIPTION_OFFSET}`;

/** A tab's one-line description, under the layout's tab heading. */
export function ServiceTabDescription({ children }: { children: ReactNode }) {
  return <p className={SERVICE_TAB_DESCRIPTION_CLASS}>{children}</p>;
}
