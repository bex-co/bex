import type { ReactNode } from "react";
import { CardHeader, CardTitle } from "@/common/components/ui/card";

/**
 * An environment card's header, shared by the ready card and its loading
 * preview so both lay out the same rows (w4/715). The title line is one 22px
 * row at every width: the name truncates rather than wrapping, and the
 * Protected badge and resource count never shrink. Where the header is
 * narrower than 28rem (a 390px phone, a 768px tablet beside the sidebar) the
 * actions drop to their own row under it, so the name keeps the full width;
 * a squeezed beside-the-actions column stacked "production" one letter per
 * line. Wider, the actions sit at the right of the title line.
 */
export function EnvironmentCardHeader({
  title,
  actions,
  titleAs = "h2",
}: {
  /** The name, badge and count (or their skeleton bars). */
  title: ReactNode;
  /** Manage resources and the kebab (or their skeleton boxes). */
  actions: ReactNode;
  /** `div` for the loading preview, which names nothing yet. */
  titleAs?: "h2" | "div";
}) {
  // CardHeader is the size container, so the layout that answers its width
  // has to live one element in.
  return (
    <CardHeader className="block">
      <div className="flex flex-col gap-3 @md/card-header:flex-row @md/card-header:items-center @md/card-header:justify-between">
        <CardTitle
          as={titleAs}
          className="flex h-[1.375rem] min-w-0 items-center gap-2"
        >
          {title}
        </CardTitle>
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      </div>
    </CardHeader>
  );
}
