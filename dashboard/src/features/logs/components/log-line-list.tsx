import { memo, useCallback, useLayoutEffect, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
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

// Estimated row height before measurement — one `text-xs` line at
// `leading-relaxed` plus the row's vertical padding. Dynamic measurement
// (`virtualizer.measureElement`) refines each rendered row afterward, so a
// wrapped multi-line row still lays out at its true height.
const ROW_ESTIMATE = 24;

// Rows rendered above/below the viewport so a fast scroll doesn't flash blank.
const OVERSCAN = 12;

// Vertical inset (previously the row container's `p-3` top/bottom padding),
// applied through the virtualizer so it survives windowing at the very top and
// very bottom of the list rather than being lost with the unrendered rows.
const LIST_INSET = 12;

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
 *
 * The rows are **virtualized** (`@tanstack/react-virtual`, w9/m83): only the
 * visible window (plus overscan) is in the DOM, so a busy live tail no longer
 * pays a DOM-reconciliation cost proportional to the whole retained buffer —
 * the remaining per-frame cost after w9/m63 moved ANSI parsing to ingest and
 * memoized each row. The mounted rows are still `LogRow`-memoized, so a window
 * shift or append re-renders only the rows that actually changed.
 *
 * Text-selection tradeoff (the known cost of virtualization): selecting and
 * copying works within the on-screen window, but a drag-select can no longer
 * span rows that have scrolled out of the DOM. Whole-buffer copy is not
 * supported; the retained buffer is available to the reader by scrolling. This
 * is the deliberate, documented tradeoff for bounding the DOM on the platform's
 * most-watched screen.
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

  // Keyed by the line's dedupe key so a row's measured height survives appends
  // and reorders (a wrapped line keeps its true height when new lines arrive).
  const getItemKey = useCallback((index: number) => lines[index].key, [lines]);

  const virtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => viewportRef.current,
    estimateSize: () => ROW_ESTIMATE,
    getItemKey,
    overscan: OVERSCAN,
    paddingStart: LIST_INSET,
    paddingEnd: LIST_INSET,
  });

  // Scroll the newest line to the viewport bottom through the virtualizer (which
  // knows the total measured height even though most rows aren't in the DOM),
  // rather than poking `scrollTop = scrollHeight` on the short windowed content.
  const scrollToBottom = useCallback(() => {
    if (lines.length > 0) {
      virtualizer.scrollToIndex(lines.length - 1, { align: "end" });
    }
  }, [lines.length, virtualizer]);

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
  }, [lines, pinned, scrollToBottom]);

  const virtualRows = virtualizer.getVirtualItems();
  const totalSize = virtualizer.getTotalSize();
  // Spacer heights standing in for the unrendered rows above and below the
  // window — this keeps every row in normal flow (so wrap/nowrap layout and
  // in-window text selection behave exactly as before), unlike absolute
  // positioning which would also break the nowrap horizontal scroll.
  const paddingTop = virtualRows.length > 0 ? virtualRows[0].start : 0;
  const paddingBottom =
    virtualRows.length > 0
      ? totalSize - virtualRows[virtualRows.length - 1].end
      : 0;

  return (
    <div className={cn("relative", fill && "h-full")}>
      <div
        ref={viewportRef}
        data-log-viewport=""
        onScroll={onScroll}
        style={fill ? undefined : { height: VIEWPORT_HEIGHT }}
        className={cn(
          "overflow-auto rounded-md border bg-muted/30 font-mono text-xs leading-relaxed",
          fill && "h-full",
        )}
      >
        <div className={cn("px-3", wrap ? "min-w-full" : "w-max min-w-full")}>
          <div aria-hidden style={{ height: paddingTop }} />
          {virtualRows.map((virtualRow) => (
            <LogRow
              key={virtualRow.key}
              index={virtualRow.index}
              measureRef={virtualizer.measureElement}
              line={lines[virtualRow.index]}
              wrap={wrap}
              showTimestamps={showTimestamps}
              onInstanceFilter={onInstanceFilter}
              t={t}
            />
          ))}
          <div aria-hidden style={{ height: paddingBottom }} />
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

// A single log row, memoized so a window shift or an appended line re-renders
// only the rows that changed — the ring buffer keeps line objects referentially
// stable, and the remaining props are primitives or stable callbacks (`t` is
// useCallback-stable, changing only with the language, which should re-render
// every row's aria-label; `measureRef` is the virtualizer's stable bound
// method). `index`/`measureRef` wire the row into the virtualizer's dynamic
// measurement (w9/m83) — `data-index` is how it identifies the measured row.
const LogRow = memo(function LogRow({
  index,
  measureRef,
  line,
  wrap,
  showTimestamps,
  onInstanceFilter,
  t,
}: {
  index: number;
  measureRef: (node: Element | null) => void;
  line: LogLine;
  wrap: boolean;
  showTimestamps: boolean;
  onInstanceFilter?: (instance: string) => void;
  t: Translate;
}) {
  return (
    <div
      data-index={index}
      ref={measureRef}
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

// Public instance ids are `<service-id>-<opaque>` (20-char base32-hex suffix).
// Prefer a compact suffix when present; otherwise show the full id. Filtering
// always uses the full backend-provided value from the line / chip.
function shortInstance(instance: string): string {
  const suffix = instance.split("-").at(-1) ?? instance;
  if (/^[0-9a-v]{20}$/.test(suffix)) {
    return suffix.slice(0, 5);
  }
  return suffix.length === 5 ? suffix : instance;
}
