import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/common/lib/utils/utils";
import {
  useLogPanelHeight,
  type LogPanelFill,
} from "@/features/logs/hooks/use-log-panel-height";

/**
 * A log view's state where its lines would be — no lines in range, a failed
 * read, a search that timed out — drawn in the log panel's own box: its
 * border, tint and screen-filling height, so the page keeps the shape it has
 * while loading and once lines land, rather than collapsing to a short card.
 */
export function LogPanelState({
  fillViewport,
  icon: Icon,
  tone = "muted",
  title,
  body,
  detail,
  action,
  role,
}: {
  /** As the panel's own `fillViewport`. */
  fillViewport: LogPanelFill;
  icon: LucideIcon;
  tone?: "muted" | "destructive";
  title: string;
  body: ReactNode;
  /** The server's own words, for support, in mono under the body. */
  detail?: string;
  /** The way forward: Try again, Turn on Live, Clear filters. */
  action?: ReactNode;
  /** `alert` for a failure, so assistive technology announces it. */
  role?: "alert";
}) {
  const { ref, height } = useLogPanelHeight<HTMLDivElement>(fillViewport);
  return (
    <div
      ref={ref}
      role={role}
      style={{ height }}
      className="flex flex-col items-center justify-center rounded-md border bg-muted/30 px-6 text-center"
    >
      <div
        className={cn(
          "mb-4 flex size-10 items-center justify-center rounded-full",
          tone === "destructive"
            ? "bg-destructive/10 text-destructive"
            : "bg-muted text-muted-foreground",
        )}
      >
        <Icon className="size-5" aria-hidden="true" />
      </div>
      <p className="font-medium">{title}</p>
      <p className="mt-1 max-w-sm text-sm text-balance text-muted-foreground">
        {body}
      </p>
      {detail ? (
        <p className="mt-2 max-w-full truncate font-mono text-xs text-muted-foreground">
          {detail}
        </p>
      ) : null}
      {action ? (
        <div className="mt-4 flex flex-wrap justify-center gap-2">{action}</div>
      ) : null}
    </div>
  );
}
