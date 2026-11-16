import type React from "react";
import type { ReactNode } from "react";
import {
  SECTIONED_BODY_CLASS,
  SECTIONED_GRID_CLASS,
  SECTION_NAVIGATION_STICKY_CLASS,
} from "@/common/components/section-navigation";
import { cn } from "@/common/lib/utils/utils";

/**
 * Page frames: the scroller, the content width, the page header and, with a
 * section navigation, the rail / sticky-bar layout (section-navigation.tsx).
 *
 * Layout follows the content area, not the viewport: the sidebar is resizable,
 * so the scroller is the `page` container and the body column an unnamed
 * `@container` that cards query with `@md:` / `@3xl:`.
 *
 * A route's pending component renders the same frame with `pending` set, so
 * the header, navigation and width are the ready page's own and the swap only
 * fills in the body. `pending` is the `data-route-skeleton` id the skeleton
 * tests and slow-navigation sweep read; the header slot is the `page-header`
 * region.
 */

export type PageFrameWidth = "standard" | "form" | "wide";

const PAGE_FRAME_WIDTH: Record<PageFrameWidth, string> = {
  standard: "max-w-5xl",
  form: "max-w-2xl",
  /** Data-dense pages whose tables want the room: Overview, Router. */
  wide: "max-w-7xl",
};

/**
 * The page scroller. It is the `page` container the layout's container
 * queries measure, and it publishes its top padding as `--page-pad-top` so a
 * sticky bar inside can cancel it and sit flush under the app header (sticky
 * offsets are measured inside the scroller's padding).
 */
const PAGE_SCROLL_CLASS =
  "@container/page min-h-0 flex-1 overflow-auto p-4 [--page-pad-top:1rem] sm:p-6 sm:[--page-pad-top:1.5rem]";

/** The page scroller, for a page that lays out its own column (a detail page
 *  under its resource header). `data-page-scroll` and the class travel
 *  together: section navigation finds the scroller by the attribute and
 *  cancels its padding through `--page-pad-top`. */
export function PageScroll({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-page-scroll=""
      className={cn(PAGE_SCROLL_CLASS, className)}
      {...props}
    />
  );
}

export function PageHeader({
  title,
  description,
  actions,
  titleAddon,
  leading,
  className,
}: {
  title: ReactNode;
  /** Before the title block: a back button. */
  leading?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  /** Inline next to the title: a badge, a status, a beta tag. */
  titleAddon?: ReactNode;
  className?: string;
}) {
  return (
    <div
      data-page-header=""
      className={cn(
        "flex flex-wrap items-start justify-between gap-x-4 gap-y-3",
        className,
      )}
    >
      <div className="flex min-w-0 items-start gap-3">
        {leading ? <div className="shrink-0">{leading}</div> : null}
        <div className="min-w-0 space-y-1">
          <div className="flex min-h-8 flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
            {titleAddon}
          </div>
          {description ? (
            // A div, not a p: a description may be a loading bar or a line
            // with its own block content.
            <div className="text-muted-foreground max-w-prose text-sm">
              {description}
            </div>
          ) : null}
        </div>
      </div>
      {actions ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {actions}
        </div>
      ) : null}
    </div>
  );
}

export function PageFrame({
  width = "standard",
  header,
  navigation,
  pending,
  className,
  children,
  "data-testid": testId,
}: {
  "data-testid"?: string;
  width?: PageFrameWidth;
  /** Usually a `<PageHeader>`. */
  header?: ReactNode;
  /** A section navigation; turns the frame into the settings layout. */
  navigation?: ReactNode;
  /** Route skeleton id: renders this frame as that route's pending state. */
  pending?: string;
  className?: string;
  children?: ReactNode;
}) {
  const pendingProps = pending
    ? {
        inert: true,
        "aria-hidden": true as const,
        "data-skeleton-frame": "true",
        "data-route-skeleton": pending,
      }
    : {};
  // Only the body pulses while pending: the header and navigation are the
  // ready page's own static copy, so they hold still.
  const bodyPulse = pending ? "animate-pulse motion-reduce:animate-none" : "";
  const headerSlot = header ? (
    <div
      className="min-w-0 @4xl/page:col-start-1 @4xl/page:row-start-1"
      data-skeleton-region={pending ? "page-header" : undefined}
    >
      {header}
    </div>
  ) : null;

  return (
    <PageScroll
      data-testid={testId}
      className={cn(pending && "group", className)}
      {...pendingProps}
    >
      {navigation ? (
        <div
          className={cn(
            "mx-auto",
            SECTIONED_GRID_CLASS,
            PAGE_FRAME_WIDTH[width],
          )}
        >
          {headerSlot}
          <div
            className={cn(
              SECTION_NAVIGATION_STICKY_CLASS,
              header && "@4xl/page:row-span-2",
            )}
            data-skeleton-region={pending ? "navigation" : undefined}
          >
            {navigation}
          </div>
          <div className={cn(SECTIONED_BODY_CLASS, bodyPulse)}>{children}</div>
        </div>
      ) : (
        <div
          className={cn("mx-auto w-full space-y-6", PAGE_FRAME_WIDTH[width])}
        >
          {headerSlot}
          <div className={cn("@container space-y-6", bodyPulse)}>
            {children}
          </div>
        </div>
      )}
    </PageScroll>
  );
}
