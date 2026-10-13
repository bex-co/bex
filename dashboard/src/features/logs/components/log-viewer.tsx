import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { AlertCircle, WifiOff } from "lucide-react";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useUrlEcho } from "@/common/hooks/use-url-echo";
import { EmptyState } from "@/common/components/empty-state";
import { LogEmptyState } from "./log-empty-state";
import { useLogHistory } from "../hooks/use-log-history";
import {
  useLiveLogs,
  type EventSourceFactory,
  type LiveStatus,
} from "../hooks/use-live-logs";
import { mergeLogLines } from "../lib/map";
import {
  EMPTY_LOG_FILTERS,
  LOG_PAGE_SIZE,
  LOG_TEXT_MODE_DISPLAY,
  STRUCTURED_FILTER_KEYS,
  hasActiveLogFilters,
  storeOnlyFilterKey,
  usesStoreOnlyFilters,
  type LogFilters,
} from "../types";
import { LogFilterBar } from "./log-filter-bar";
import { LogLineList } from "./log-line-list";
import { LogTruncationNotice } from "./log-truncation-notice";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { DEFAULT_LOG_RANGE } from "../lib/log-search";

interface LogViewerProps {
  resource: string;
  /** Injectable SSE factory — threaded through for tests. */
  createEventSource?: EventSourceFactory;
  range?: RangeSelection;
  onRangeChange?: (range: RangeSelection) => void;
  /**
   * The filter + live state the URL names (w7/m42). The viewer starts from it
   * and adopts it again whenever a navigation it did not write changes it — a
   * nav link, back/forward (w4/m204). Its own writes echoing back are left
   * alone, so they can't clobber text typed since.
   */
  urlFilters?: LogFilters;
  urlLive?: boolean;
  /**
   * URL sync (w7/m42): fired with the debounced filter + live state after each
   * real change (never at mount — that state was just derived from the URL) so
   * the route can mirror the state into the query string.
   */
  onFiltersChange?: (filters: LogFilters, live: boolean) => void;
}

/**
 * The service page's Logs tab: bex-api's historical `logs(...)` query plus an
 * SSE live tail (docs/ADR010-observability.md), laid out like Render's Logs viewer
 * (filter bar + line list). Owns the filter state; the data layer (history hook
 * + live hook) is presentation-free.
 */
export function LogViewer({
  resource,
  createEventSource,
  range = DEFAULT_LOG_RANGE,
  onRangeChange = () => undefined,
  urlFilters = EMPTY_LOG_FILTERS,
  urlLive = true,
  onFiltersChange,
}: LogViewerProps) {
  const { t } = useTranslations();
  const urlKey = logStateKey(urlFilters, urlLive);
  const [state, setState] = useState(() => viewFromUrl(urlFilters, urlLive));
  const urlEcho = useUrlEcho(urlKey);

  // Adopt a foreign URL during render, and use the adopted state for the rest
  // of this pass: the history query reobserves while rendering, and the range
  // prop changes in the same pass, so one render with the source's filters
  // would query them against the destination's range.
  let shown = state;
  if (urlEcho.foreign) {
    shown = viewFromUrl(urlFilters, urlLive);
    setState(shown);
  }
  const { filters, live, committed } = shown;
  urlEcho.showing(logStateKey({ ...filters, ...committed }, live));

  // An adopted URL sets each draft and its committed value together, which
  // cancels a timer still pending for text typed before the navigation.
  const { text: committedText, path: committedPath } = committed;
  const { text: draftText, path: draftPath } = filters;
  useEffect(() => {
    if (draftText === committedText) return;
    const id = setTimeout(
      () => setState((previous) => commitDraft(previous, "text", draftText)),
      COMMIT_DELAY_MS,
    );
    return () => clearTimeout(id);
  }, [draftText, committedText]);
  useEffect(() => {
    if (draftPath === committedPath) return;
    const id = setTimeout(
      () => setState((previous) => commitDraft(previous, "path", draftPath)),
      COMMIT_DELAY_MS,
    );
    return () => clearTimeout(id);
  }, [draftPath, committedPath]);

  // Scalar deps (not the filters object, whose identity churns per keystroke)
  // keep queryFilters' identity stable while typing, so the URL-sync effect
  // below fires only on settled values.
  const { type, level, method, statusCode, instance } = filters;
  const queryFilters = useMemo<LogFilters>(
    () => ({
      type,
      level,
      method,
      statusCode,
      instance,
      text: committedText,
      path: committedPath,
    }),
    [type, level, method, statusCode, instance, committedText, committedPath],
  );

  // Mirror the committed state into the URL (w7/m42). The first run is
  // skipped — the mount state was just derived from the URL, and navigating
  // during hydration triggers a full document reload loop under TanStack
  // Start (observed live). The ref keeps the route's inline callback out of
  // the sync effect's deps so only real filter/live changes fire; it is
  // refreshed in its own every-render effect (declared first, so it has run
  // by the time the sync effect fires in the same flush). A URL change fires
  // it too: an echo that a newer change has overtaken is answered with the
  // current state, so the URL converges on what the viewer shows.
  const onFiltersChangeRef = useRef(onFiltersChange);
  useEffect(() => {
    onFiltersChangeRef.current = onFiltersChange;
  });
  const syncedOnce = useRef(false);
  useEffect(() => {
    if (!syncedOnce.current) {
      syncedOnce.current = true;
      return;
    }
    onFiltersChangeRef.current?.(queryFilters, live);
  }, [queryFilters, live, urlKey]);

  // The live tail reads pod stdout, so it can't serve request logs or the
  // store-only filters — offer live only when none is active (instance is fine,
  // it's a pod name). A store-only filter with live on just pauses the tail.
  const liveSupported = !usesStoreOnlyFilters(queryFilters);

  // Stable identity so LogLineList's memoized rows actually skip re-rendering
  // when a live line appends (an inline arrow would defeat the memo).
  const patchFilters = useCallback(
    (patch: Partial<LogFilters>) =>
      setState((previous) => ({
        ...previous,
        filters: { ...previous.filters, ...patch },
      })),
    [setState],
  );
  const onInstanceFilter = useCallback(
    (instance: string) => patchFilters({ instance }),
    [patchFilters],
  );

  const historyWindow = useLiveRange(range);
  const history = useLogHistory(
    resource,
    queryFilters,
    historyWindow,
    JSON.stringify(range),
    // Search what the row shows: a colored message is found by its visible
    // text, not only by raw bytes split by escapes (w4/m189).
    LOG_TEXT_MODE_DISPLAY,
  );
  const stream = useLiveLogs({
    resource,
    enabled: live && liveSupported,
    type: queryFilters.type,
    text: committedText,
    textMode: LOG_TEXT_MODE_DISPLAY,
    instance: queryFilters.instance,
    // The store-only filters disable the tail instead of narrowing it, so they
    // join its reader identity: toggling one clears lines streamed for the old
    // query, while a plain Live pause keeps them on screen (w4/m192).
    readerKey: storeOnlyFilterKey(queryFilters),
    // Follow the pod from the selected window's edge, not kubelet offset 0, so a
    // narrow range no longer surfaces lines older than it on first connect
    // (w6/m111). The tail freezes this per-subscription — the sliding window
    // won't reopen the stream.
    startTime: historyWindow.startTime,
    createEventSource,
  });

  // History and live tail merged into one chronological reader, deduped — the
  // tail's in-window replay can predate the historical head (w4/m192).
  const lines = useMemo(
    () =>
      mergeLogLines(
        history.lines,
        stream.lines,
        Date.parse(historyWindow.startTime),
      ),
    [history.lines, stream.lines, historyWindow.startTime],
  );

  const filtered = hasActiveLogFilters(queryFilters);

  // One body per state (store-unavailable / error / loading-first / empty /
  // list) — resolved to a single node so the render stays flat.
  let body: ReactNode;
  if (history.storeUnavailable && history.lines.length === 0) {
    // Request logs / structured filters need the durable store, which isn't
    // wired here (local dev). An explanatory state, not a generic error.
    body = (
      <EmptyState
        iconName="Database"
        title={t("logs.storeRequiredTitle")}
        description={t("logs.storeRequiredBody")}
      />
    );
  } else if (history.timedOut && history.lines.length === 0) {
    body = <LogEmptyState reason="timeout" />;
  } else if (history.error && history.lines.length === 0) {
    body = (
      <LogEmptyState
        reason="failed"
        title={t("logs.errorTitle")}
        message={history.error.message}
      />
    );
  } else if (history.loading && history.lines.length === 0) {
    body = <LogPanelSkeleton />;
  } else if (lines.length === 0) {
    body = (
      <LogEmptyState
        reason="empty"
        filtered={filtered}
        emptyTitle={t("logs.emptyTitle")}
        emptyBody={t("logs.emptyBody")}
        pages={history}
      />
    );
  } else {
    body = (
      <div className="space-y-2">
        {history.error ? (
          <div
            role="alert"
            className="rounded-md border border-destructive/30 px-3 py-2 text-sm text-destructive"
          >
            {t("logs.errorTitle")}: {history.error.message}
          </div>
        ) : null}
        {history.hasMore ? (
          <LogTruncationNotice
            partial={history.lines.length < LOG_PAGE_SIZE}
            loadingOlder={history.loadingOlder}
            onLoadOlder={history.loadOlder}
          />
        ) : null}
        {live && liveSupported && stream.status === "error" ? (
          <div className="flex items-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
            <WifiOff className="h-4 w-4" />
            {t("logs.disconnected")}
          </div>
        ) : null}
        <LogLineList
          lines={lines}
          onInstanceFilter={onInstanceFilter}
          hasMore={history.hasMore}
          loadingOlder={history.loadingOlder}
          onLoadOlder={history.loadOlder}
        />
        <StreamStatus
          live={live && liveSupported}
          liveSupported={liveSupported}
          status={stream.status}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <LogFilterBar
        resource={resource}
        filters={filters}
        onChange={patchFilters}
        live={live}
        onLiveChange={(next) =>
          setState((previous) => ({ ...previous, live: next }))
        }
        liveSupported={liveSupported}
        range={range}
        onRangeChange={onRangeChange}
      />
      <p className="text-xs text-muted-foreground">
        {t("logs.rangeHistoryNote")}
      </p>
      {body}
    </div>
  );
}

// How long a free-text draft waits before it reaches the queries and the URL,
// so a keystroke doesn't refetch history and reopen the stream.
const COMMIT_DELAY_MS = 300;
// `filters` is what the controls show; its free-text fields are drafts.
// `committed` is the text and path the queries and the URL carry.
interface LogViewState {
  filters: LogFilters;
  live: boolean;
  committed: { text: string; path: string };
}

function viewFromUrl(filters: LogFilters, live: boolean): LogViewState {
  return {
    filters,
    live,
    committed: { text: filters.text, path: filters.path },
  };
}

/** One comparable value for a whole filter + live state. */
function logStateKey(filters: LogFilters, live: boolean) {
  return JSON.stringify([
    filters.type,
    filters.text,
    ...STRUCTURED_FILTER_KEYS.map((key) => filters[key]),
    live,
  ]);
}

function commitDraft(
  state: LogViewState,
  field: "text" | "path",
  value: string,
): LogViewState {
  return { ...state, committed: { ...state.committed, [field]: value } };
}

// The footer under the log list: a live pulse while streaming, a paused note
// when the tail is off, and a note when the active filters can't be live-tailed.
function StreamStatus({
  live,
  liveSupported,
  status,
}: {
  live: boolean;
  liveSupported: boolean;
  status: LiveStatus;
}) {
  const { t } = useTranslations();
  return (
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
      {!liveSupported ? (
        <>
          <AlertCircle className="h-3.5 w-3.5" />
          {t("logs.liveUnsupported")}
        </>
      ) : live && status === "open" ? (
        <>
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-500 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
          </span>
          {t("logs.streaming")}
        </>
      ) : !live ? (
        <>
          <AlertCircle className="h-3.5 w-3.5" />
          {t("logs.paused")}
        </>
      ) : null}
    </p>
  );
}
