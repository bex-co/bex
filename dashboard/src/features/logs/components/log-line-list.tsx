import {
  memo,
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useId,
  type Ref,
} from "react";
import {
  Bug,
  CircleX,
  Info,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
import { useVirtualizer } from "@tanstack/react-virtual";
import { InstantTooltip } from "@/common/components/instant-tooltip";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  formatClockTime,
  formatMediumDate,
  formatMonthDay,
} from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  useLogPanelHeight,
  type LogPanelFill,
} from "@/features/logs/hooks/use-log-panel-height";
import { type AnsiSpan } from "../lib/ansi";
import { lineLevel, lineLevelText, type LineLevel } from "../lib/log-levels";
import {
  clampMessage,
  logLineText,
  logListRows,
  logRowsText,
  shortInstance,
  type LogOrder,
} from "../lib/log-rows";
import { type LogResourceLabel } from "../lib/resource-labels";
import { LOG_TYPE_REQUEST, type LogLine } from "../types";
import {
  HeldLinesButton,
  NewLogsInLiveTailButton,
  ReturnButton,
} from "./log-list-buttons";
import {
  LOG_ROW_MENU_ATTRIBUTE,
  LogRowMenu,
  type LogRowMenuItems,
} from "./log-row-menu";

type Translate = ReturnType<typeof useTranslations>["t"];

// A request (HTTP access) line's status chip, tinted by response class — the
// at-a-glance signal Render's request-log rows lead with. An unknown/empty code
// falls back to the neutral chip.
function statusChipClass(status: string): string {
  switch (status.charAt(0)) {
    case "2":
      return "bg-green-500/15 text-green-800 dark:text-green-400";
    case "3":
      return "bg-blue-500/15 text-blue-700 dark:text-blue-400";
    case "4":
      return "bg-amber-500/15 text-amber-800 dark:text-amber-400";
    case "5":
      return "bg-red-500/15 text-red-700 dark:text-red-400";
    default:
      return "bg-muted text-muted-foreground";
  }
}

// How close to the bottom (px) still counts as "pinned" — a small slack so a
// sub-pixel scroll position or a wrapped final line doesn't unpin autoscroll.
const PIN_THRESHOLD = 24;

// How close to the top (px) counts as "load older" — large enough that a fast
// flick to the top still fires before the user stares at empty overscan.
const LOAD_OLDER_THRESHOLD = 64;

// Estimated row height before measurement — one `text-xs` line at
// `leading-relaxed` plus the row's vertical padding. Dynamic measurement
// (`virtualizer.measureElement`) refines each rendered row afterward, so a
// wrapped multi-line row still lays out at its true height.
const ROW_ESTIMATE = 24;

// A row's tint and edge rule by its level: error and warning lines stand
// out from the stream, as on Render's log.
const LEVEL_ROW_CLASS: Partial<Record<LineLevel, string>> = {
  error: "bg-destructive/10 shadow-[inset_2px_0_0_var(--color-destructive)]",
  warning: "bg-amber-500/10 shadow-[inset_2px_0_0_var(--color-amber-500)]",
};

// The level gutter's mark (w3/m509): Render's icon per level, its info mark
// shown only while the row is hovered or under the keyboard's cursor.
const LEVEL_ICONS: Record<LineLevel, LucideIcon> = {
  debug: Bug,
  info: Info,
  notice: Info,
  warning: TriangleAlert,
  error: CircleX,
};
const LEVEL_ICON_CLASS: Record<LineLevel, string> = {
  debug: "text-muted-foreground",
  info: "text-muted-foreground opacity-0 group-hover/row:opacity-100 group-data-active/row:opacity-100",
  notice: "text-blue-700 dark:text-blue-400",
  warning: "text-amber-800 dark:text-amber-400",
  error: "text-red-700 dark:text-red-400",
};
// One line of the row's text tall, so the mark centres on its first line
// whatever the message wraps to.
const GUTTER_CLASS = "flex h-[1.625em] w-3.5 shrink-0 items-center self-start";

// The end note's line (`h-6`), counted into the content a panel grows to.
const END_NOTE_HEIGHT = 24;
// How far past its screen-sized height an `ended` log's panel grows.
const GROW_TO_FIT_LIMIT = 1.25;

// Rows rendered above/below the viewport so a fast scroll doesn't flash blank.
const OVERSCAN = 12;

/** How far the reader is from the list's bottom edge. */
const fromBottom = (el: HTMLElement) =>
  el.scrollHeight - el.scrollTop - el.clientHeight;

/** What a log explorer's toolbar and shortcuts drive in its list (w3/m509). */
export interface LogLineListHandle {
  /** Home and End: the list's first or last row. */
  jump: (edge: "top" | "bottom") => void;
  /** The rows shown, as Options ▸ Copy logs writes them. */
  text: () => string;
}

/**
 * A display reflow (wrap or timestamps toggled) keeps re-placing the reader's
 * row while rows remeasure: until this long after the last pass, and never
 * longer than SETTLE_MAX_MS, so a busy live tail cannot keep it settling
 * (w4/m228).
 */
const SETTLE_MS = 250;
const SETTLE_MAX_MS = 1000;

/** What a display reflow keeps in view: a row and offset, or the newest line. */
type SettleTarget = { key: string; offset: number } | "newest";

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
  /**
   * Size the panel to the viewport's room below it (min 320px) instead of a
   * fixed 520px: the service Logs tab, where the panel is the page and its
   * stream status line sits under it. `"flush"` leaves only the page's bottom
   * padding (a deploy's log, which has no status line). Other hosts (a
   * datastore's log) keep the fixed height.
   */
  fillViewport?: LogPanelFill;
  /**
   * The log is finished and will not grow (a finished deploy's): it closes
   * with an "End of log" line, so a short one reads as complete rather than
   * still loading, and one that only just overflows the panel (up to a
   * quarter past its height) grows it to show whole rather than scroll to
   * hide a row. The panel never shrinks, so a loading placeholder at its
   * height still holds.
   */
  ended?: boolean;
  /** More history exists older than the loaded pages (w4/m107). */
  hasMore?: boolean;
  loadingOlder?: boolean;
  /** Fired when the user scrolls near the top of the pane. */
  onLoadOlder?: () => void;
  /**
   * A project view's resources (w3/m505): each line then names its own after
   * the time, with Render's colored dot and `[name]` linking to the resource.
   * Absent on a single resource's log, whose rows are unchanged.
   */
  resourceLabels?: ReadonlyMap<string, LogResourceLabel>;
  /** Options ▸ Order by: ascending (the newest line at the bottom, the
   *  default) or descending (the newest at the top, older pages below). */
  order?: LogOrder;
  /** Options ▸ Expand everything: no message is clamped. */
  expandAll?: boolean;
  /** The explorer's handle on the list: its shortcuts and Copy logs. */
  handleRef?: Ref<LogLineListHandle>;
  /**
   * A live tail of runtime lines (w3/m528): while the reader is away from
   * the newest line, lines newer than the ones shown are held behind
   * Render's "New logs" button instead of joining the list under them. A
   * deploy's log and a paused explorer leave it off.
   */
  holdWhileAway?: boolean;
  /** The row menu's View in context and SSH into instance (w3/m528); every
   *  row has Copy log line. Keep it stable: the rows are memoized on it. */
  rowMenu?: LogRowMenuItems;
  /**
   * The line a View in context link names (w3/m528): marked, and scrolled to
   * the middle of the panel once it is loaded (older pages are read until it
   * is, or none is left). The list then stays where the reader puts it.
   */
  markedKey?: string;
  /** Render's "New logs in Live Tail": a paused explorer's tail has a new
   *  line. Absent while there is none, or once dismissed. */
  newLogsInLiveTail?: { onSwitch: () => void; onDismiss: () => void };
}

/**
 * What a held list shows (w3/m528): the lines it showed when the reader left
 * the newest line, and any older page loaded since. Every other line is held.
 */
interface HeldView {
  /** The keys shown when the reader left. */
  keys: ReadonlySet<string>;
  /** The oldest of them, in epoch milliseconds: anything older is a page. */
  oldest: number;
}

function holdView(lines: readonly LogLine[]): HeldView {
  let oldest = Infinity;
  for (const line of lines) {
    const time = Date.parse(line.timestamp);
    if (time < oldest) oldest = time;
  }
  return { keys: new Set(lines.map((line) => line.key)), oldest };
}

/** The lines a held list shows, or every line when none of the ones it
 *  showed is left (a cleared or expired buffer has nothing to hold under). */
function shownWhileHeld(lines: LogLine[], view: HeldView): LogLine[] {
  let kept = 0;
  const shown = lines.filter((line) => {
    if (view.keys.has(line.key)) {
      kept += 1;
      return true;
    }
    return Date.parse(line.timestamp) < view.oldest;
  });
  return kept === 0 ? lines : shown;
}

/**
 * The monospace log line list (Render's layout: level · timestamp ·
 * [instance] · message, newest at the bottom unless Options ▸ Order by says
 * descending). Autoscrolls to the newest line while the user is pinned to it;
 * scrolling away releases the pin (so reading history isn't yanked away) and
 * shows Render's Return to Bottom (Return to Top descending). The
 * wrap/timestamp toggles (w9/003) and the order and Expand everything options
 * (w3/m509) are display-only knobs an explorer's options menu drives; each
 * defaults to today's behavior.
 *
 * Render's explorer pieces (w3/m528): a row menu (…) on the hovered row or
 * the keyboard's (Shift+F10), a live tail that holds new lines behind a "New
 * logs" button while the reader is away, a paused explorer's "New logs in
 * Live Tail", and View in context's marked line.
 *
 * Each row leads with its level's mark (Render's gutter: warning and error
 * rows tinted, the info mark shown on hover), its time opens the full local,
 * UTC and Unix timestamp on hover (or under the keyboard's row cursor), and a
 * list spanning several days marks each with a date row (w3/m509).
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
 * span rows that have scrolled out of the DOM. The whole buffer is copied
 * through the handle's `text()` instead (Options ▸ Copy logs, `⌘/Ctrl ⇧ C`,
 * w3/m509), which writes every row shown, not only the mounted window.
 */
export function LogLineList({
  lines,
  onInstanceFilter,
  wrap = true,
  showTimestamps = true,
  fill = false,
  fillViewport = false,
  ended = false,
  hasMore = false,
  loadingOlder = false,
  onLoadOlder,
  resourceLabels,
  order = "asc",
  expandAll = false,
  handleRef,
  holdWhileAway = false,
  rowMenu,
  markedKey,
  newLogsInLiveTail,
}: LogLineListProps) {
  const { t, i18n } = useTranslations();
  const language = i18n.language;
  const keysHintId = useId();
  const viewportRef = useRef<HTMLDivElement>(null);
  const descending = order === "desc";
  // Pinned: the reader is at the newest line, the bottom ascending and the
  // top descending. A View in context link opens on its marked line instead.
  const [pinned, setPinned] = useState(() => !markedKey);
  // Held lines (w3/m528): the hold lives here, not in each explorer, because
  // only the list knows when the reader is away; the explorers say whether
  // their lines are a live tail's. Captured from the lines shown in the
  // render the reader left in, so nothing newer slips in under them.
  const holding = holdWhileAway && !pinned;
  const [held, setHeld] = useState<HeldView | null>(null);
  let heldView = held;
  if (holding && held === null) {
    heldView = holdView(lines);
    setHeld(heldView);
  } else if (!holding && held !== null) {
    heldView = null;
    setHeld(null);
  }
  const shownLines = useMemo(
    () => (heldView ? shownWhileHeld(lines, heldView) : lines),
    [lines, heldView],
  );
  const heldCount = lines.length - shownLines.length;
  const rows = useMemo(
    () => logListRows(shownLines, order),
    [shownLines, order],
  );
  // The keyboard's row cursor in the focused list, by row key so it stays on
  // its line as lines arrive or the order flips: ↑/↓ move it, Enter shows
  // only its line's instance (the tags themselves are out of the Tab order),
  // Escape lets it go.
  const [cursor, setCursor] = useState<string | null>(null);
  // The row under the pointer and the row whose menu is open (w3/m528): the
  // row menu mounts on those and the cursor's row only, so an appended or
  // scrolled-in row never pays for one. One hovered row at a time, so a tap
  // on a phone (whose pointer never leaves) shows one row's button.
  const [hoveredKey, setHoveredKey] = useState<string | null>(null);
  const [menuKey, setMenuKey] = useState<string | null>(null);
  // The menu opened from the keyboard returns focus to the list, not to its
  // button, so ↑/↓ keep moving the cursor.
  const keyboardMenu = useRef(false);
  const onMenuOpenChange = useCallback(
    (key: string, open: boolean) =>
      setMenuKey((current) => (open ? key : current === key ? null : current)),
    [],
  );
  const onMenuCloseAutoFocus = useCallback((event: Event) => {
    if (!keyboardMenu.current) return;
    keyboardMenu.current = false;
    event.preventDefault();
    viewportRef.current?.focus();
  }, []);
  const { copy: copyText } = useCopyToClipboard({
    successText: t("logs.lineCopied"),
    errorText: t("logs.lineCopyFailed"),
  });
  const copyLine = useCallback(
    (text: string) => void copyText(text),
    [copyText],
  );
  // Rows whose clamped message the reader opened (w3/m509).
  const [expandedRows, setExpandedRows] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const toggleExpanded = useCallback(
    (key: string) =>
      setExpandedRows((previous) => {
        const next = new Set(previous);
        if (!next.delete(key)) next.add(key);
        return next;
      }),
    [],
  );
  // Whether rows are scrolled off the top, which fade out (below).
  const [scrolledDown, setScrolledDown] = useState(false);
  const anchorRef = useRef<{ key: string; offset: number } | null>(null);
  const previousRows = useRef(rows);
  // A wrap, timestamp, order or expand toggle changes every row's height or
  // place while the records stay the same. Until the rows have remeasured,
  // the reader's row is kept in place by key, and transient geometry may
  // neither replace that target nor unpin the reader (w4/m228).
  const display = `${wrap}:${showTimestamps}:${order}:${expandAll}`;
  const previousDisplay = useRef(display);
  const settling = useRef<SettleTarget | null>(null);
  const settleDeadline = useRef(0);
  const settleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const loadOlderRef = useRef(onLoadOlder);
  const loadingOlderRef = useRef(loadingOlder);
  const hasMoreRef = useRef(hasMore);
  // A layout effect, declared before the ones that read these refs, so that
  // they see this render's pager: an explorer's loadOlder is rebuilt with its
  // state, and the previous one returns early while its page loads (w3/m528).
  useLayoutEffect(() => {
    loadOlderRef.current = onLoadOlder;
    loadingOlderRef.current = loadingOlder;
    hasMoreRef.current = hasMore;
  });

  // Keyed by the line's dedupe key so a row's measured height survives appends
  // and reorders (a wrapped line keeps its true height when new lines arrive).
  const getItemKey = useCallback((index: number) => rows[index].key, [rows]);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    estimateSize: () => ROW_ESTIMATE,
    getItemKey,
    overscan: OVERSCAN,
    paddingStart: LIST_INSET,
    paddingEnd: LIST_INSET,
    // Jumping to the newest row scrolls on past the inset (and any end
    // note), so the newest line never sits on the panel's bottom border.
    scrollPaddingEnd: LIST_INSET,
  });

  // Scroll to the list's first row or its last through the virtualizer
  // (which knows the total measured height even though most rows aren't in
  // the DOM), rather than poking `scrollTop = scrollHeight` on the short
  // windowed content.
  const scrollToEdge = useCallback(
    (edge: "top" | "bottom") => {
      if (rows.length === 0) return;
      if (edge === "top") {
        if (viewportRef.current) viewportRef.current.scrollTop = 0;
      } else {
        virtualizer.scrollToIndex(rows.length - 1, { align: "end" });
      }
    },
    [rows.length, virtualizer],
  );
  const scrollToNewest = useCallback(
    () => scrollToEdge(descending ? "top" : "bottom"),
    [scrollToEdge, descending],
  );

  // The row at the top of the viewport and how far into it the reader is.
  const recordAnchor = useCallback(
    (el: HTMLElement) => {
      const visible = virtualizer
        .getVirtualItems()
        .find((row) => row.end > el.scrollTop);
      if (visible) {
        anchorRef.current = {
          key: String(visible.key),
          offset: el.scrollTop - visible.start,
        };
      }
    },
    [virtualizer],
  );
  // Recompute the pin from the scroll position; releasing it while the user
  // reads away from the newest line, restoring it the moment they return.
  const recordPin = useCallback(
    (el: HTMLElement) =>
      setPinned((descending ? el.scrollTop : fromBottom(el)) <= PIN_THRESHOLD),
    [descending],
  );

  // Put a row back at the top, its offset clamped to the row's current height
  // (a row that became shorter cannot keep an offset past its end). False when
  // the row is no longer in the buffer.
  const placeRow = (
    el: HTMLElement,
    anchor: { key: string; offset: number },
  ): boolean => {
    const index = rows.findIndex((row) => row.key === anchor.key);
    if (index < 0) return false;
    const position = virtualizer.getOffsetForIndex(index, "start");
    if (!position) return false;
    const size = virtualizer.measurementsCache[index].size;
    el.scrollTop = Math.max(
      0,
      position[0] + Math.min(anchor.offset, Math.max(0, size - 1)),
    );
    return true;
  };

  const onScroll = () => {
    const el = viewportRef.current;
    if (el) {
      setScrolledDown(el.scrollTop > 0);
      // A one-line list scrolls sideways; the row menu's button keeps to the
      // panel's right edge through this, with no render (see LogRow).
      el.style.setProperty("--log-scroll-left", `${el.scrollLeft}px`);
    }
    if (!el || settling.current) return;
    recordAnchor(el);
    recordPin(el);
    // Near the oldest edge — the top ascending, the bottom descending —
    // request the next older page (w4/m107).
    if (
      (descending ? fromBottom(el) : el.scrollTop) <= LOAD_OLDER_THRESHOLD &&
      hasMoreRef.current &&
      !loadingOlderRef.current
    ) {
      loadOlderRef.current?.();
    }
  };

  // Settled: what the reader now sees, in the final geometry, becomes the
  // anchor and pin that later appends and scrolls work from.
  const endSettling = useCallback(() => {
    if (settleTimer.current) clearTimeout(settleTimer.current);
    if (!settling.current) return;
    settling.current = null;
    virtualizer.shouldAdjustScrollPositionOnItemSizeChange = undefined;
    const el = viewportRef.current;
    if (el) {
      recordAnchor(el);
      recordPin(el);
    }
  }, [recordAnchor, recordPin, virtualizer]);
  useEffect(
    () => () => {
      if (settleTimer.current) clearTimeout(settleTimer.current);
    },
    [],
  );

  // Keep the visible row anchored when pages arrive or old rows expire.
  // Total-height deltas also include new head/SSE rows, so they shift an
  // unpinned reader even when nothing changed above the viewport.
  useLayoutEffect(() => {
    const el = viewportRef.current;
    if (!el) return;
    if (previousDisplay.current !== display) {
      previousDisplay.current = display;
      const target = pinned ? "newest" : anchorRef.current;
      if (target) {
        settling.current = target;
        settleDeadline.current = Date.now() + SETTLE_MAX_MS;
        // The restore below places the row; the virtualizer's own size-change
        // compensation would fight it.
        virtualizer.shouldAdjustScrollPositionOnItemSizeChange = () => false;
      }
      // Old-mode sizes are wrong. Remeasure the mounted rows now: a row whose
      // height did not change gets no resize notification and would
      // otherwise fall back to the estimate.
      virtualizer.measure();
      virtualizer.elementsCache.forEach((row) => {
        if (row.isConnected) virtualizer.measureElement(row);
      });
    }
    const target = settling.current;
    if (target) {
      if (target === "newest") scrollToNewest();
      else placeRow(el, target);
      if (settleTimer.current) clearTimeout(settleTimer.current);
      const remaining = settleDeadline.current - Date.now();
      settleTimer.current = setTimeout(
        endSettling,
        Math.max(0, Math.min(SETTLE_MS, remaining)),
      );
      previousRows.current = rows;
      return;
    }
    const anchor = anchorRef.current;
    if (!pinned && anchor && previousRows.current !== rows) {
      if (!placeRow(el, anchor)) el.scrollTop = 0;
    } else {
      // The virtualizer may publish its new visible range after onScroll.
      recordAnchor(el);
    }
    previousRows.current = rows;
  });

  // useLayoutEffect so the scroll lands before paint — no visible jump as new
  // lines arrive.
  useLayoutEffect(() => {
    if (pinned) scrollToNewest();
  }, [rows, pinned, scrollToNewest]);

  // View in context (w3/m528): the marked line goes to the middle of the
  // panel once, as soon as it is loaded; until then older pages are read.
  // After that the reader's scrolling is left alone.
  const placedMark = useRef<string | undefined>(undefined);
  useLayoutEffect(() => {
    if (!markedKey || placedMark.current === markedKey) return;
    if (rows.some((row) => row.key === markedKey)) return;
    if (hasMore && !loadingOlder) loadOlderRef.current?.();
  }, [markedKey, rows, hasMore, loadingOlder]);
  // The virtualizer lays out no rows until it has measured the panel. Before
  // then the list has no height to scroll in: the mark's scroll would land at
  // the top and the pin would read the reader as at the newest line.
  const virtualRows = virtualizer.getVirtualItems();
  const laidOut = virtualRows.length > 0;
  useLayoutEffect(() => {
    if (!markedKey || placedMark.current === markedKey || !laidOut) return;
    const index = rows.findIndex((row) => row.key === markedKey);
    if (index < 0) return;
    placedMark.current = markedKey;
    virtualizer.scrollToIndex(index, { align: "center" });
    // A short list does not scroll, and so sends no scroll event: the pin is
    // read from where the list now is.
    if (viewportRef.current) recordPin(viewportRef.current);
  }, [markedKey, rows, laidOut, virtualizer, recordPin]);

  const dayLabel = useCallback(
    (day: number) =>
      new Date(day).getFullYear() === new Date().getFullYear()
        ? formatMonthDay(day, language)
        : formatMediumDate(day, language),
    [language],
  );

  useImperativeHandle(
    handleRef,
    () => ({
      jump: (edge) => {
        if (rows.length === 0) return;
        endSettling();
        scrollToEdge(edge);
        // The newest edge pins at once; leaving it unpins on the scroll it
        // causes (a log that fits its panel never scrolls, and stays pinned).
        if ((edge === "top") === descending) setPinned(true);
      },
      text: () =>
        logRowsText(rows, {
          time: showTimestamps
            ? (timestamp) => formatClockTime(timestamp, language)
            : null,
          day: dayLabel,
          resource: resourceLabels
            ? (id) => resourceLabels.get(id)?.name
            : undefined,
        }),
    }),
    [
      rows,
      descending,
      endSettling,
      scrollToEdge,
      showTimestamps,
      language,
      dayLabel,
      resourceLabels,
    ],
  );

  // A descending list ends at its oldest loaded line, so it says the log ends
  // there only once no older page is left.
  const showEnd = ended && !(descending && hasMore);
  const { ref: panelRef, height: fillHeight } =
    useLogPanelHeight<HTMLDivElement>(fill ? false : fillViewport);
  const totalSize = virtualizer.getTotalSize();
  // The rows, their inset and any end note, plus the viewport's 1px top and
  // bottom border.
  const contentHeight = totalSize + (showEnd ? END_NOTE_HEIGHT : 0) + 2;
  const panelHeight =
    ended &&
    contentHeight > fillHeight &&
    contentHeight <= fillHeight * GROW_TO_FIT_LIMIT
      ? contentHeight
      : fillHeight;
  // Spacer heights standing in for the unrendered rows above and below the
  // window — this keeps every row in normal flow (so wrap/nowrap layout and
  // in-window text selection behave exactly as before), unlike absolute
  // positioning which would also break the nowrap horizontal scroll.
  const paddingTop = virtualRows.length > 0 ? virtualRows[0].start : 0;
  const paddingBottom =
    virtualRows.length > 0
      ? totalSize - virtualRows[virtualRows.length - 1].end
      : 0;

  // The keyboard cursor's next line row from `from` in `step`'s direction,
  // passing over date rows; `from` itself when there is none.
  const nextLine = (from: number, step: 1 | -1) => {
    for (let i = from + step; i >= 0 && i < rows.length; i += step) {
      if (rows[i].kind === "line") return i;
    }
    return from;
  };
  // Return to Bottom/Top and the held-lines button: back to the newest line,
  // which releases any held lines. Outside the viewport, so their pointer
  // does not end a reflow's settling; returning must (w4/m228).
  const returnToNewest = () => {
    endSettling();
    setPinned(true);
    scrollToNewest();
  };
  const loadingOlderNote = loadingOlder ? (
    <p className="px-3 py-2 text-center text-muted-foreground">
      {t("logs.loadingOlder")}
    </p>
  ) : null;

  return (
    <div ref={panelRef} className={cn("relative", fill && "h-full")}>
      {/* A focusable, named region: keyboard readers scroll the list with the
          arrow and Page keys, as a mouse wheel does. Its listeners only end a
          reflow's settling on reader input; it is not a control. */}
      {/* eslint-disable-next-line jsx-a11y-x/no-noninteractive-element-interactions -- see above */}
      <div
        ref={viewportRef}
        role="region"
        aria-label={t("logs.linesLabel")}
        // eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex -- a scrollable region must be reachable by keyboard (WCAG 2.1.1)
        tabIndex={0}
        data-log-viewport=""
        onScroll={onScroll}
        // Any reader input ends a display reflow's settling at once.
        onWheel={endSettling}
        onTouchStart={endSettling}
        onPointerDown={endSettling}
        onKeyDown={(event) => {
          // A row menu's keys reach here through its portal; they are the
          // menu's own (w3/m528).
          if (!event.currentTarget.contains(event.target as Node)) return;
          endSettling();
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            const step = event.key === "ArrowDown" ? 1 : -1;
            const current =
              cursor === null
                ? -1
                : rows.findIndex((row) => row.key === cursor);
            let next: number;
            if (current < 0) {
              // The first move lands on the first or last line in view.
              const visible = virtualizer.getVirtualItems();
              const edge =
                step === 1
                  ? (visible[0]?.index ?? 0)
                  : (visible.at(-1)?.index ?? rows.length - 1);
              next = rows[edge]?.kind === "line" ? edge : nextLine(edge, step);
            } else {
              next = nextLine(current, step);
            }
            const row = rows[next];
            if (row?.kind !== "line") return;
            setCursor(row.key);
            virtualizer.scrollToIndex(next, { align: "auto" });
          } else if (event.key === "Enter" && cursor !== null) {
            const row = rows.find((candidate) => candidate.key === cursor);
            const instance = row?.kind === "line" ? row.line.instance : "";
            if (instance && onInstanceFilter) {
              event.preventDefault();
              onInstanceFilter(instance);
            }
          } else if (
            (event.key === "F10" && event.shiftKey) ||
            event.key === "ContextMenu"
          ) {
            // The row menu of the line under the cursor (w3/m528).
            if (cursor === null) return;
            event.preventDefault();
            keyboardMenu.current = true;
            setMenuKey(cursor);
          } else if (event.key === "Escape") {
            setCursor(null);
          }
        }}
        onBlur={(event) => {
          // Focus going into the cursor row's menu keeps the cursor.
          const next = event.relatedTarget;
          if (
            next instanceof Element &&
            next.closest(`[${LOG_ROW_MENU_ATTRIBUTE}]`)
          ) {
            return;
          }
          setCursor(null);
        }}
        onPointerLeave={(event) => {
          // A touch's pointer leaves when the finger lifts: the tapped row
          // keeps its button until another row is tapped.
          if (event.pointerType !== "touch") setHoveredKey(null);
        }}
        aria-describedby={keysHintId}
        style={fill ? undefined : { height: panelHeight }}
        className={cn(
          // The list anchors the reader itself; the browser's own overflow
          // anchoring would move it again mid-reflow (w4/m228).
          // A container, so a wrapped row can stack by the panel's own width.
          "peer @container/logs overflow-auto rounded-md border bg-muted/30 font-mono text-xs leading-relaxed [overflow-anchor:none] focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
          fill && "h-full",
        )}
      >
        {/* Rows scrolled off the top fade out under the border rather than
            being sliced through by it: a sticky wash in the panel's own
            colour (its muted tint over the page), above the rows. Solid for
            its first few pixels, so a tinted row's last sliver never shows
            as a strip along the edge. */}
        {scrolledDown ? (
          <div
            aria-hidden
            className="pointer-events-none sticky top-0 left-0 z-10 -mb-6 h-6 bg-linear-to-b from-[color-mix(in_oklab,var(--color-muted)_30%,var(--color-background))] from-40% to-transparent"
          />
        ) : null}
        {/* Older pages load at the oldest edge: the top ascending, the
            bottom descending. */}
        {descending ? null : loadingOlderNote}
        <div className={wrap ? "min-w-full" : "w-max min-w-full"}>
          <div aria-hidden style={{ height: paddingTop }} />
          {virtualRows.map((virtualRow) => {
            const row = rows[virtualRow.index];
            return row.kind === "day" ? (
              <DayRow
                key={virtualRow.key}
                index={virtualRow.index}
                measureRef={virtualizer.measureElement}
                label={dayLabel(row.day)}
              />
            ) : (
              <LogRow
                key={virtualRow.key}
                index={virtualRow.index}
                active={cursor === row.key}
                hovered={hoveredKey === row.key}
                menuOpen={menuKey === row.key}
                marked={markedKey === row.key}
                onHover={setHoveredKey}
                onMenuOpenChange={onMenuOpenChange}
                onMenuCloseAutoFocus={onMenuCloseAutoFocus}
                onCopyLine={copyLine}
                rowMenu={rowMenu}
                measureRef={virtualizer.measureElement}
                line={row.line}
                wrap={wrap}
                showTimestamps={showTimestamps}
                onInstanceFilter={onInstanceFilter}
                resourceLabel={
                  resourceLabels &&
                  (resourceLabels.get(row.line.resource) ?? null)
                }
                expandAll={expandAll}
                expanded={expandedRows.has(row.key)}
                onToggleExpanded={toggleExpanded}
                t={t}
                language={language}
              />
            );
          })}
          <div aria-hidden style={{ height: paddingBottom }} />
        </div>
        {descending ? loadingOlderNote : null}
        {showEnd ? (
          <p className="sticky left-0 flex h-6 w-full items-center gap-3 px-4 font-sans text-[11px] text-muted-foreground before:h-px before:flex-1 before:bg-border before:content-[''] after:h-px after:flex-1 after:bg-border after:content-['']">
            {t("logs.endOfLog")}
          </p>
        ) : null}
      </div>

      <p id={keysHintId} className="sr-only">
        {onInstanceFilter
          ? t("logs.linesKeysHint")
          : t("logs.linesKeysHintMenu")}
      </p>
      {/* The keys, shown to a sighted keyboard user while the region has
          focus: in a corner over the oldest line as it fades out (the top
          ascending, the bottom descending), never over the newest, where a
          live tail's lines and the floating buttons land. */}
      <p
        aria-hidden
        className={cn(
          "pointer-events-none absolute right-2 z-20 hidden max-w-[calc(100%-1rem)] truncate rounded border bg-background px-2 py-0.5 text-xs text-muted-foreground shadow-xs peer-focus-visible:block",
          descending ? "bottom-2" : "top-2",
        )}
      >
        {onInstanceFilter
          ? t("logs.linesKeysShort")
          : t("logs.linesKeysShortMenu")}
      </p>
      {!pinned ? (
        <ReturnButton descending={descending} onClick={returnToNewest} />
      ) : null}
      {heldCount > 0 ? (
        <HeldLinesButton
          count={heldCount}
          descending={descending}
          onClick={returnToNewest}
        />
      ) : null}
      {newLogsInLiveTail ? (
        <NewLogsInLiveTailButton
          descending={descending}
          onSwitch={newLogsInLiveTail.onSwitch}
          onDismiss={newLogsInLiveTail.onDismiss}
        />
      ) : null}
    </div>
  );
}

// The date row over a new day's first line (Render's `Nov 19`), in a list
// spanning more than one day. Measured like a line row, so the virtualizer
// places the rows under it at their true offsets.
function DayRow({
  index,
  measureRef,
  label,
}: {
  index: number;
  measureRef: (node: Element | null) => void;
  label: string;
}) {
  return (
    <div
      data-index={index}
      data-log-day=""
      ref={measureRef}
      className="flex items-center px-4 pt-2 pb-1 font-sans text-[11px] font-medium text-muted-foreground"
    >
      {/* Held at the panel's left edge while a nowrap list scrolls across. */}
      <span className="sticky left-4">{label}</span>
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
  active,
  hovered,
  menuOpen,
  marked,
  onHover,
  onMenuOpenChange,
  onMenuCloseAutoFocus,
  onCopyLine,
  rowMenu,
  measureRef,
  line,
  wrap,
  showTimestamps,
  onInstanceFilter,
  resourceLabel,
  expandAll,
  expanded,
  onToggleExpanded,
  t,
  language,
}: {
  index: number;
  /** The keyboard's row cursor is on this row. */
  active: boolean;
  /** The pointer is on this row (the list's one hovered row). */
  hovered: boolean;
  /** This row's menu is open. */
  menuOpen: boolean;
  /** The line a View in context link names (w3/m528). */
  marked: boolean;
  onHover: (key: string) => void;
  onMenuOpenChange: (key: string, open: boolean) => void;
  onMenuCloseAutoFocus: (event: Event) => void;
  onCopyLine: (text: string) => void;
  rowMenu: LogRowMenuItems | undefined;
  measureRef: (node: Element | null) => void;
  line: LogLine;
  wrap: boolean;
  showTimestamps: boolean;
  onInstanceFilter?: (instance: string) => void;
  /** The line's resource in a project view (see LogLineList.resourceLabels):
   *  null for one the list does not name, which shows its id; undefined when
   *  the list names no resources at all. */
  resourceLabel: LogResourceLabel | null | undefined;
  /** Options ▸ Expand everything is on. */
  expandAll: boolean;
  /** The reader opened this row's clamped message. */
  expanded: boolean;
  onToggleExpanded: (key: string) => void;
  t: Translate;
  /** The dashboard language the line clock reads in. */
  language: string;
}) {
  const isRequest = line.type === LOG_TYPE_REQUEST;
  const level = lineLevel(line);
  // A long message shows its first lines until the reader opens it.
  const clip = expandAll ? null : clampMessage(line);
  const shown = clip && !expanded ? clip : line;
  return (
    <div
      data-index={index}
      data-active={active ? "" : undefined}
      data-marked={marked ? "" : undefined}
      ref={measureRef}
      onPointerEnter={() => onHover(line.key)}
      className={cn(
        // Every cell reads from the first line: a tag centred against a
        // wrapped message sat a line below its time.
        "group/row relative flex items-baseline gap-3 px-4 py-0.5 hover:bg-muted/60",
        // The keyboard's row cursor (↑/↓ in the focused list).
        active && "outline-2 -outline-offset-2 outline-ring",
        // Errors and warnings stand out as Render's log does: a tinted row
        // with a rule at its edge; the text keeps its colour.
        level ? LEVEL_ROW_CLASS[level] : undefined,
        // Render highlights the row whose menu is open, and the line a View
        // in context link names.
        menuOpen && "bg-muted/60",
        marked &&
          "bg-primary/10 shadow-[inset_2px_0_0_var(--color-primary)] hover:bg-primary/15",
        // In a narrow panel a wrapped row puts its time and instance on one
        // line and gives the message the panel's full width under them:
        // beside a fixed time-and-instance gutter, the message had under
        // half a phone's width and broke tokens mid-word ("2026-/07"). A row
        // with an instance tag has the wider gutter, so it stacks sooner.
        wrap
          ? line.instance
            ? "whitespace-pre-wrap break-words @max-lg/logs:flex-wrap @max-lg/logs:gap-x-2 @max-lg/logs:gap-y-0 @max-lg/logs:py-1"
            : "whitespace-pre-wrap break-words @max-md/logs:flex-wrap @max-md/logs:gap-x-2 @max-md/logs:gap-y-0 @max-md/logs:py-1"
          : "whitespace-pre",
      )}
    >
      <LevelMark
        level={level}
        text={level ? lineLevelText(line, level) : ""}
        t={t}
      />
      {showTimestamps ? (
        <LineTime
          timestamp={line.timestamp}
          active={active}
          language={language}
        />
      ) : null}
      {resourceLabel !== undefined && line.resource ? (
        <ResourceTag id={line.resource} label={resourceLabel} />
      ) : null}
      {line.instance ? (
        onInstanceFilter ? (
          <button
            type="button"
            // Part of the line's text, not a control row: exempt from the
            // touch region's 44px control height, with its own 44px-tall
            // hit area on touch widths instead. Out of the Tab order: one
            // stop per line made the log hundreds of stops long; the focused
            // list's Enter (and Filters ▸ Instance) is the keyboard's way to
            // the same filter.
            data-touch-exempt=""
            tabIndex={-1}
            className="relative shrink-0 cursor-pointer rounded-sm px-0.5 text-muted-foreground underline decoration-muted-foreground/50 decoration-dotted underline-offset-2 hover:bg-muted hover:text-foreground max-lg:after:absolute max-lg:after:-inset-y-3.5 max-lg:after:-inset-x-0 max-lg:after:content-['']"
            aria-label={t("logs.filterByInstance", {
              instance: line.instance,
            })}
            title={line.instance}
            onClick={() => onInstanceFilter(line.instance)}
          >
            [{shortInstance(line.instance)}]
          </button>
        ) : (
          // Short like the button's label: a full datastore instance id
          // (`red-<id>-<opaque>`) left no room for the message on a narrow
          // screen, which wrapped one character per line (w5/m139).
          <span
            className="shrink-0 text-muted-foreground"
            title={line.instance}
          >
            [{shortInstance(line.instance)}]
          </span>
        )
      ) : null}
      {/* A request line leads with method/status chips from its labels
          (w5/008), then the host and path Render's request rows show before
          the line's own summary (w4/m325). */}
      {isRequest && line.method ? (
        <span className="shrink-0 rounded bg-muted px-1 font-semibold text-muted-foreground">
          {line.method}
        </span>
      ) : null}
      {isRequest && line.statusCode ? (
        <span
          className={cn(
            "shrink-0 rounded px-1 font-semibold tabular-nums",
            statusChipClass(line.statusCode),
          )}
        >
          {line.statusCode}
        </span>
      ) : null}
      <span
        className={cn(
          "min-w-0 flex-1 text-foreground",
          // Wrapped lines hang 2ch in, so a continuation reads as part of its
          // entry, not a new one.
          wrap && "pl-[2ch] -indent-[2ch]",
          wrap &&
            (line.instance
              ? "@max-lg/logs:basis-full"
              : "@max-md/logs:basis-full"),
        )}
      >
        {/* Inside the message cell, so a long host or path wraps with the
            message rather than squeezing it (w5/m139). */}
        {isRequest && (line.host || line.path)
          ? `${line.host}${line.path} `
          : null}
        <LogMessage
          spans={shown.spans}
          text={"text" in shown ? shown.text : shown.message}
        />
        {clip ? (
          <>
            {expanded ? " " : "… "}
            <ExpandToggle
              label={expanded ? t("logs.showLess") : t("logs.showMore")}
              onClick={() => onToggleExpanded(line.key)}
            />
          </>
        ) : null}
      </span>
      {hovered || active || menuOpen ? (
        <LogRowMenu
          line={line}
          text={logLineText(
            line,
            // A project view's resource, by name, or by id when the list
            // does not name it.
            resourceLabel === undefined
              ? undefined
              : (resourceLabel?.name ?? line.resource),
          )}
          items={rowMenu}
          open={menuOpen}
          onOpenChange={(open) => onMenuOpenChange(line.key, open)}
          onCopy={onCopyLine}
          onCloseAutoFocus={onMenuCloseAutoFocus}
          t={t}
          // At the row's top right; a one-line row scrolls sideways, so its
          // button keeps to the panel's visible right edge instead.
          className={wrap ? "right-1" : undefined}
          style={wrap ? undefined : ONE_LINE_MENU_STYLE}
        />
      ) : null}
    </div>
  );
});

// The row menu's place in a one-line list, whose rows run past the panel:
// the panel's width from the row's start, plus how far it has scrolled
// sideways (`--log-scroll-left`, set on scroll), less the button.
const ONE_LINE_MENU_STYLE = {
  left: "calc(var(--log-scroll-left, 0px) + 100cqw - 1.75rem)",
};

// Render's level gutter (w3/m509): the line's level as an icon, its word on
// hover and as its accessible name; an empty cell of the same width for a
// line with no level, so every row's time starts in one column.
function LevelMark({
  level,
  text,
  t,
}: {
  level: LineLevel | null;
  /** The level's word: the hover title and accessible name. */
  text: string;
  t: Translate;
}) {
  if (!level) return <span aria-hidden className={GUTTER_CLASS} />;
  const Icon = LEVEL_ICONS[level];
  return (
    <span
      role="img"
      aria-label={t("logs.lineLevel", { level: text })}
      title={text}
      className={cn(GUTTER_CLASS, LEVEL_ICON_CLASS[level])}
    >
      <Icon aria-hidden className="size-3.5" />
    </span>
  );
}

// A row's time. The full local, UTC and Unix timestamp opens on hover, as on
// Render's log, or under the keyboard's row cursor (the time is out of the
// Tab order, like the instance tag). The tooltip mounts only then: a Radix
// tooltip in every row cost each appended or scrolled-in row its mount.
function LineTime({
  timestamp,
  active,
  language,
}: {
  timestamp: string;
  active: boolean;
  language: string;
}) {
  const [hovered, setHovered] = useState(false);
  const className = "shrink-0 tabular-nums text-muted-foreground";
  // An empty cell for a line without a time keeps the row's gaps the same.
  if (!timestamp) return <span className={className} />;
  const time = formatClockTime(timestamp, language);
  if (!hovered && !active) {
    return (
      <time
        dateTime={timestamp}
        className={className}
        onPointerEnter={() => setHovered(true)}
      >
        {time}
      </time>
    );
  }
  return (
    <InstantTooltip
      value={timestamp}
      open={active}
      initiallyOpen={hovered}
      tabIndex={-1}
      className={cn(className, "no-underline")}
    >
      {time}
    </InstantTooltip>
  );
}

// Opens or closes one row's clamped message: part of the line's text, so
// exempt from the touch region's control height like the instance tag.
function ExpandToggle({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      data-touch-exempt=""
      onClick={onClick}
      className="cursor-pointer rounded-sm font-sans text-[11px] text-muted-foreground underline underline-offset-2 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      {label}
    </button>
  );
}

// A project view line's resource (w3/m505): Render's colored dot and
// `[name]`, linking to the resource's own page. A resource the page's list
// does not name (one that joined the view since its history was read) shows
// its id, unlinked, until the next read names it.
function ResourceTag({
  id,
  label,
}: {
  id: string;
  label: LogResourceLabel | null;
}) {
  const className =
    "inline-flex shrink-0 items-center gap-1.5 text-muted-foreground";
  const content = (
    <>
      <span
        aria-hidden
        className={cn(
          "size-2 shrink-0 rounded-full",
          label?.dotClass ?? "bg-muted-foreground/40",
        )}
      />
      [{label?.name ?? id}]
    </>
  );
  const linkClass = cn(
    className,
    "rounded-sm underline-offset-2 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
  );
  return label?.href ? (
    <Link to={label.href} data-touch-exempt="" className={linkClass}>
      {content}
    </Link>
  ) : (
    <span className={className}>{content}</span>
  );
}

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
