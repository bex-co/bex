import { memo, useLayoutEffect, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import { type AnsiSpan } from "../lib/ansi";
import { LOG_TYPE_REQUEST, type LogLine } from "../types";

type Translate = ReturnType<typeof useTranslations>["t"];

// A request (HTTP access) line's status chip, tinted by response class — the
// at-a-glance signal Render's request-log rows lead with. An unknown/empty code
// falls back to the neutral chip.
function statusChipClass(status: string): string {
  switch (status.charAt(0)) {
    case "2":
      return "bg-green-500/15 text-green-700 dark:text-green-400";
    case "3":
      return "bg-blue-500/15 text-blue-700 dark:text-blue-400";
    case "4":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-400";
    case "5":
      return "bg-red-500/15 text-red-700 dark:text-red-400";
    default:
      return "bg-muted text-muted-foreground";
  }
}

// How close to the bottom (px) still counts as "pinned" — a small slack so a
// sub-pixel scroll position or a wrapped final line doesn't unpin autoscroll.
const PIN_THRESHOLD = 24;

// Height of the scroll viewport; the list fills it and scrolls internally.
const VIEWPORT_HEIGHT = 520;

interface LogLineListProps {
  lines: LogLine[];
  /** Select an application-log instance as a filter. When supplied, pod names
   * are presented as their short Kubernetes instance slug. */
  onInstanceFilter?: (instance: string) => void;
  /** Wrap long lines (default). false => single-line rows with horizontal scroll. */
  wrap?: boolean;
  /** Show the per-line timestamp column (default). */
  showTimestamps?: boolean;
  /** Fill the parent's height instead of the fixed viewport (the deploy
   *  page's maximized mode, w9/003). The parent owns the height. */
  fill?: boolean;
}

/**
 * The monospace log line list (Render's layout: timestamp · [instance] ·
 * message, newest at the bottom). Autoscrolls to the newest line while the user
 * is pinned to the bottom; scrolling up releases the pin (so reading history
 * isn't yanked away) and surfaces a "jump to latest" affordance. The wrap/
 * timestamp toggles (w9/003) are display-only knobs the deploy page's options
 * menu drives; both default to today's behavior.
 */
export function LogLineList({
  lines,
  onInstanceFilter,
  wrap = true,
  showTimestamps = true,
  fill = false,
}: LogLineListProps) {
  const { t } = useTranslations();
  const viewportRef = useRef<HTMLDivElement>(null);
  const [pinned, setPinned] = useState(true);

  const scrollToBottom = () => {
    const el = viewportRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  // Recompute the pin from the scroll position; releasing it while the user
  // reads up, restoring it the moment they return to the bottom.
  const onScroll = () => {
    const el = viewportRef.current;
    if (!el) return;
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
    setPinned(distance <= PIN_THRESHOLD);
  };

  // useLayoutEffect so the scroll lands before paint — no visible jump as new
  // lines append.
  useLayoutEffect(() => {
    if (pinned) scrollToBottom();
  }, [lines, pinned]);

  return (
    <div className={cn("relative", fill && "h-full")}>
      <div
        ref={viewportRef}
        onScroll={onScroll}
        style={fill ? undefined : { height: VIEWPORT_HEIGHT }}
        className={cn(
          "overflow-auto rounded-md border bg-muted/30 font-mono text-xs leading-relaxed",
          fill && "h-full",
        )}
      >
        <div className={cn("p-3", wrap ? "min-w-full" : "w-max min-w-full")}>
          {lines.map((line) => (
            <LogRow
              key={line.key}
              line={line}
              wrap={wrap}
              showTimestamps={showTimestamps}
              onInstanceFilter={onInstanceFilter}
              t={t}
            />
          ))}
        </div>
      </div>

      {!pinned ? (
        <Button
          size="sm"
          variant="secondary"
          className="absolute bottom-3 left-1/2 -translate-x-1/2 shadow-md"
          onClick={() => {
            setPinned(true);
            scrollToBottom();
          }}
        >
          <ArrowDown className="h-4 w-4" />
          {t("logs.jumpToLatest")}
        </Button>
      ) : null}
    </div>
  );
}

// A single log row, memoized so an appended line re-renders only itself —
// the ring buffer keeps line objects referentially stable, and the remaining
// props are primitives or stable callbacks (t is useCallback-stable, changing
// only with the language, which should re-render every row's aria-label).
const LogRow = memo(function LogRow({
  line,
  wrap,
  showTimestamps,
  onInstanceFilter,
  t,
}: {
  line: LogLine;
  wrap: boolean;
  showTimestamps: boolean;
  onInstanceFilter?: (instance: string) => void;
  t: Translate;
}) {
  return (
    <div
      className={cn(
        "flex gap-3 px-1 py-0.5 hover:bg-muted/60",
        wrap ? "whitespace-pre-wrap break-words" : "whitespace-pre",
      )}
    >
      {showTimestamps ? (
        <span className="shrink-0 tabular-nums text-muted-foreground">
          {line.time}
        </span>
      ) : null}
      {line.instance ? (
        onInstanceFilter ? (
          <button
            type="button"
            className="shrink-0 cursor-pointer rounded-sm text-muted-foreground/70 underline-offset-2 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={t("logs.filterByInstance", {
              instance: line.instance,
            })}
            title={line.instance}
            onClick={() => onInstanceFilter(line.instance)}
          >
            [{shortInstance(line.instance)}]
          </button>
        ) : (
          <span className="shrink-0 text-muted-foreground/70">
            [{line.instance}]
          </span>
        )
      ) : null}
      {/* A request line leads with method/status chips from its labels
          (w5/008) — the at-a-glance structure Render's request rows show,
          instead of the raw Traefik JSON getting app-line treatment. */}
      {line.type === LOG_TYPE_REQUEST && line.method ? (
        <span className="shrink-0 rounded bg-muted px-1 font-semibold text-muted-foreground">
          {line.method}
        </span>
      ) : null}
      {line.type === LOG_TYPE_REQUEST && line.statusCode ? (
        <span
          className={cn(
            "shrink-0 rounded px-1 font-semibold tabular-nums",
            statusChipClass(line.statusCode),
          )}
        >
          {line.statusCode}
        </span>
      ) : null}
      <span className="min-w-0 flex-1 text-foreground">
        <LogMessage spans={line.spans} text={line.message} />
      </span>
    </div>
  );
});

// The message cell. `spans` is null for a line with nothing to interpret, which
// renders the bare string — byte-identical to the pre-ANSI DOM for the common
// app-log line. Text always goes through React's escaping; the only inline
// styles are colors this module builds from parsed integers.
function LogMessage({
  spans,
  text,
}: {
  spans: AnsiSpan[] | null;
  text: string;
}) {
  if (!spans) return text;
  return spans.map((span, i) => (
    <span key={i} className={span.className || undefined} style={span.style}>
      {span.text}
    </span>
  ));
}

// Deployment-managed pods end in Kubernetes's five-character replica slug
// (`service-<replicaset>-bv612`). Render leads with the same compact instance
// shape; keep the full pod name in the callback/title so filtering remains
// exact and only abbreviate the presentation when that suffix is present.
function shortInstance(instance: string): string {
  const suffix = instance.split("-").at(-1) ?? instance;
  return suffix.length === 5 ? suffix : instance;
}
