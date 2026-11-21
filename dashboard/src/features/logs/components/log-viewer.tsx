import { parseSearchOperators } from "@/features/logs/lib/log-search-operators";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { AlertCircle, WifiOff } from "lucide-react";
import { PlanBillingLink } from "@/common/components/plan-billing-link";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  planFeatureRequired,
  relayableReason,
} from "@/common/lib/graphql-error";
import { useUrlEcho } from "@/common/hooks/use-url-echo";
import { cn } from "@/common/lib/utils/utils.ts";
import { EmptyState } from "@/common/components/empty-state";
import { LogEmptyState } from "./log-empty-state";
import { useOpenDocked } from "../hooks/use-open-docked";
import { Button } from "@/common/components/ui/button";
import { useLogHistory } from "../hooks/use-log-history";
import {
  useLiveLogs,
  useNewLogsWatch,
  type LogSubscription,
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
  lineFilterKey,
  liveTailUnsupported,
  type LogFilters,
  type LogLine,
  withoutRequestFilters,
} from "../types";
import { LogClearedState, LogExplorerActions } from "./log-explorer";
import { useClearedLines, useLogExplorer } from "../hooks/use-log-explorer";
import { LogFilterBar } from "./log-filter-bar";
import { LogLineList } from "./log-line-list";
import type { LogLineShellTarget, LogRowMenuItems } from "./log-row-menu";
import {
  LOG_FILTER_ACTIONS_CLASS,
  LOG_MAXIMIZED_BODY_CLASS,
  LOG_MAXIMIZED_LIST_CLASS,
} from "./log-toolbar-layout";
import { LogTruncationNotice } from "./log-truncation-notice";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { RANGE_PRESETS } from "@/features/metrics/lib/range";
import {
  DEFAULT_LOG_RANGE,
  LIVE_TAIL,
  historyRange,
  isLiveTail,
  isPresetRange,
  logRangeEndMs,
  type LogRange,
} from "../lib/log-range";
import { LogThemeFrame } from "./log-theme-frame";

interface LogViewerProps {
  resource: string;
  /** Injectable SSE factory — threaded through for tests. */
  createEventSource?: EventSourceFactory;
  /**
   * The range: Live tail tails, every other range is a snapshot (w3/m537,
   * Render's `liveMode`). Last hour unless the URL names one, as Render's
   * service Logs opens.
   */
  range?: LogRange;
  onRangeChange?: (range: LogRange) => void;
  /**
   * The filter state the URL names (w7/m42). The viewer starts from it
   * and adopts it again whenever a navigation it did not write changes it — a
   * nav link, back/forward (w4/m204). Its own writes echoing back are left
   * alone, so they can't clobber text typed since.
   */
  urlFilters?: LogFilters;
  /**
   * URL sync (w7/m42): fired with the debounced filter state after each
   * real change (never at mount — that state was just derived from the URL) so
   * the route can mirror the state into the query string.
   */
  onFiltersChange?: (filters: LogFilters) => void;
  /**
   * False for a service with no public requests: every view, whether from
   * the URL or typed, drops the request type and request-line filters
   * (LogFilterBar.servesRequests, w4/613).
   */
  servesRequests?: boolean;
  /** The line a View in context link marks (w3/m528). */
  contextLine?: string;
  /** View in context's link for a line, offered while a search or filter
   *  narrows the view (w3/m528). */
  contextHref?: (line: LogLine) => string | undefined;
  /** SSH into instance's target for a line, on a service whose Shell can
   *  open (w3/m528). */
  shell?: (line: LogLine) => LogLineShellTarget | null;
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
  range: rangeProp = DEFAULT_LOG_RANGE,
  onRangeChange = () => undefined,
  urlFilters = EMPTY_LOG_FILTERS,
  onFiltersChange,
  servesRequests = true,
  contextLine,
  contextHref,
  shell,
}: LogViewerProps) {
  const frameRef = useRef<HTMLDivElement>(null);
  useOpenDocked(frameRef);
  const { t } = useTranslations();
  const urlKey = logStateKey(urlFilters);
  const [state, setState] = useState(() => viewFromUrl(urlFilters));
  const urlEcho = useUrlEcho(urlKey);

  // Adopt a foreign URL during render, and use the adopted state for the rest
  // of this pass: the history query reobserves while rendering, and the range
  // prop changes in the same pass, so one render with the source's filters
  // would query them against the destination's range.
  let shown = state;
  if (urlEcho.foreign) {
    shown = viewFromUrl(urlFilters);
    setState(shown);
  }
  const filters = servesRequests
    ? shown.filters
    : withoutRequestFilters(shown.filters);
  const committed = servesRequests
    ? shown.committed
    : { ...shown.committed, path: "", host: "" };
  urlEcho.showing(logStateKey({ ...filters, ...committed }));

  // An adopted URL sets each draft and its committed value together, which
  // cancels a timer still pending for text typed before the navigation.
  const {
    text: committedText,
    path: committedPath,
    host: committedHost,
  } = committed;
  const { text: draftText, path: draftPath, host: draftHost } = filters;
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
  useEffect(() => {
    if (draftHost === committedHost) return;
    const id = setTimeout(
      () => setState((previous) => commitDraft(previous, "host", draftHost)),
      COMMIT_DELAY_MS,
    );
    return () => clearTimeout(id);
  }, [draftHost, committedHost]);

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
      host: committedHost,
    }),
    [
      type,
      level,
      method,
      statusCode,
      instance,
      committedText,
      committedPath,
      committedHost,
    ],
  );

  // Mirror the committed state into the URL (w7/m42). The first run is
  // skipped — the mount state was just derived from the URL, and navigating
  // during hydration triggers a full document reload loop under TanStack
  // Start (observed live). The ref keeps the route's inline callback out of
  // the sync effect's deps so only real filter changes fire; it is
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
    onFiltersChangeRef.current?.(queryFilters);
  }, [queryFilters, urlKey]);

  // Request filters stream request lines from the store poll and a level
  // streams app lines (w8/m147); only a type that cannot carry an active
  // filter cannot tail, and Live tail then reads Last hour until it can.
  const liveSupported = !liveTailUnsupported(queryFilters);
  const liveOffReasonId = useId();
  // Chosen inline: the React Compiler reads a helper's result as mutable,
  // and could then not keep the lines' memo below.
  const range =
    isLiveTail(rangeProp) && !liveSupported ? DEFAULT_LOG_RANGE : rangeProp;
  const live = isLiveTail(range);
  const customEndMs = logRangeEndMs(range);

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

  // A newly committed predicate samples the clock in this render, so its first
  // history query reaches lines already on screen (w4/306).
  const historyWindow = useLiveRange(
    historyRange(range),
    JSON.stringify([resource, queryFilters]),
    // Every range but Live tail freezes the history window (w4/399): it is a
    // snapshot, so nothing refetches, arrives or expires underneath it.
    { paused: !live },
  );
  const history = useLogHistory(
    resource,
    queryFilters,
    historyWindow,
    JSON.stringify(range),
    // Search what the row shows: a colored message is found by its visible
    // text, not only by raw bytes split by escapes (w4/m189).
    LOG_TEXT_MODE_DISPLAY,
  );
  // A plan without HTTP request logs (Hobby, w8/m138) refuses the request
  // view: a plan notice, not a load failure, and no tail to retry against it.
  // The read keeps its window, so the refusal cannot move it.
  const planRefused = planFeatureRequired(history.error, "requestLogs");
  const tailing = live && !planRefused;
  // What the tail follows, and the paused view's watch with it (w3/m528).
  const subscription: LogSubscription = {
    resource,
    type: queryFilters.type,
    text: committedText,
    textMode: LOG_TEXT_MODE_DISPLAY,
    instance: queryFilters.instance,
    // The structured filters ride the subscription so streamed lines match
    // the history beside them: the store poll honors the request filters,
    // the pod tail the level (w8/m147).
    requestFilters: queryFilters,
  };
  const stream = useLiveLogs({
    ...subscription,
    enabled: tailing,
    // The line-selecting filters join the tail's reader identity: toggling one
    // clears lines streamed for the old query, while a plain Live pause keeps
    // them on screen (w4/m192).
    readerKey: lineFilterKey(queryFilters),
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
        customEndMs,
      ),
    [history.lines, stream.lines, historyWindow.startTime, customEndMs],
  );

  // Render's explorer controls (w3/m509): Options, Maximize and the
  // shortcuts, `Shift+L` clearing the lines while the tail is live.
  // The view's identity: what a clear and the paused view's watch reset on.
  const viewKey = JSON.stringify([resource, queryFilters, range]);
  const cleared = useClearedLines(lines, tailing, viewKey);
  const { explorer, attachList, attachSearch } = useLogExplorer({
    clear: cleared.clear,
  });

  const filtered = hasActiveLogFilters(queryFilters);

  // Render's row menu (w3/m528): View in context while a search or filter
  // narrows the view, SSH into instance where the Shell can open.
  const rowMenu = useMemo<LogRowMenuItems>(
    () => ({ contextHref: filtered ? contextHref : undefined, shell }),
    [filtered, contextHref, shell],
  );
  // On a preset range, a new line brings up Render's "New logs in Live
  // Tail" (w3/m528), and it and the empty state pick Live tail (w3/m537).
  const pickLiveTail = () => onRangeChange(LIVE_TAIL);
  const watch = useNewLogsWatch({
    enabled: isPresetRange(range) && liveSupported && !planRefused,
    subscription,
    since: historyWindow.endTime,
    resetKey: viewKey,
    createEventSource,
  });

  // One body per state (store-unavailable / error / loading-first / empty /
  // list) — resolved to a single node so the render stays flat.
  const historyReason = relayableReason(history.error);
  // The stream line under a state as under lines: a failed history read or an
  // empty range says nothing about whether new lines are still arriving.
  const streamLine = tailing ? <StreamStatus status={stream.status} /> : null;
  let body: ReactNode;
  // Only the lines carry their own stream line; every other state gets it
  // under the panel below.
  let isListBody = false;
  if (planRefused) {
    body = (
      <EmptyState
        iconName="LockKeyhole"
        title={t("logs.requestPlanTitle")}
        description={
          <>
            {t("logs.requestPlanBody")} <PlanBillingLink />
          </>
        }
      />
    );
  } else if (history.storeUnavailable && history.lines.length === 0) {
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
    body = <LogEmptyState reason="timeout" fillViewport />;
  } else if (history.error && history.lines.length === 0) {
    body = (
      <LogEmptyState
        reason="failed"
        fillViewport
        title={t("logs.errorTitle")}
        error={history.error}
        onRetry={history.retry}
        liveTailing={tailing && stream.status === "open"}
      />
    );
  } else if (history.loading && history.lines.length === 0) {
    body = <LogPanelSkeleton fillViewport />;
  } else if (cleared.cleared && cleared.lines.length === 0) {
    body = <LogClearedState fillViewport onRestore={cleared.restore} />;
  } else if (lines.length === 0) {
    body = (
      <LogEmptyState
        reason="empty"
        fillViewport
        filtered={filtered}
        emptyTitle={t("logs.emptyTitle")}
        emptyBody={t("logs.emptyBodyRange")}
        pages={history}
        action={
          filtered ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                patchFilters({ ...EMPTY_LOG_FILTERS, type: filters.type })
              }
            >
              {t("logs.clearFilters")}
            </Button>
          ) : (
            <>
              {!live && liveSupported ? (
                <Button variant="outline" size="sm" onClick={pickLiveTail}>
                  {t("logs.switchToLiveTail")}
                </Button>
              ) : null}
              {/* The longer range the copy suggests, when there is one to
                  widen to. */}
              {isPresetRange(range) &&
              range.spanSeconds < DAY_RANGE.spanSeconds ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onRangeChange(DAY_RANGE)}
                >
                  {t("metrics.rangeLast24Hours")}
                </Button>
              ) : null}
            </>
          )
        }
      />
    );
  } else {
    isListBody = true;
    body = (
      <div
        className={cn(
          "space-y-2",
          explorer.maximized && LOG_MAXIMIZED_BODY_CLASS,
        )}
      >
        {history.error ? (
          <div
            role="alert"
            className="rounded-md border border-destructive/30 px-3 py-2 text-sm text-destructive"
          >
            {historyReason
              ? t("logs.errorTitleWithReason", { reason: historyReason })
              : t("logs.errorTitle")}
          </div>
        ) : null}
        {history.hasMore ? (
          <LogTruncationNotice
            partial={history.lines.length < LOG_PAGE_SIZE}
            loadingOlder={history.loadingOlder}
            onLoadOlder={history.loadOlder}
          />
        ) : null}
        {tailing && stream.status === "error" ? <StreamDisconnected /> : null}
        <div className={cn(explorer.maximized && LOG_MAXIMIZED_LIST_CLASS)}>
          <LogLineList
            fillViewport
            fill={explorer.maximized}
            lines={cleared.lines}
            onInstanceFilter={onInstanceFilter}
            hasMore={history.hasMore}
            loadingOlder={history.loadingOlder}
            onLoadOlder={history.loadOlder}
            order={explorer.order}
            display={explorer.display}
            handleRef={attachList}
            holdWhileAway={tailing}
            rowMenu={rowMenu}
            markedKey={contextLine}
            newLogsInLiveTail={
              watch.available
                ? { onSwitch: pickLiveTail, onDismiss: watch.dismiss }
                : undefined
            }
          />
        </div>
        {streamLine}
      </div>
    );
  }

  return (
    <LogThemeFrame
      ref={frameRef}
      data-log-frame=""
      className="space-y-4"
      maximized={explorer.maximized}
    >
      <LogFilterBar
        resource={resource}
        filters={filters}
        onChange={patchFilters}
        liveTailDisabled={!liveSupported || planRefused}
        liveTailReasonId={liveSupported ? undefined : liveOffReasonId}
        range={range}
        onRangeChange={onRangeChange}
        servesRequests={servesRequests}
        searchRef={attachSearch}
        actions={
          <LogExplorerActions
            explorer={explorer}
            canCopy={isListBody}
            phoneMaximize
            className={LOG_FILTER_ACTIONS_CLASS}
          />
        }
      />
      {/* Where the tail cannot follow the filters, the note says why, in
          every state: Live tail is disabled in the picker, which it
          describes, so this is its only reachable reason (w4/556). */}
      {liveSupported ? null : (
        <p
          id={liveOffReasonId}
          className="flex items-center gap-1.5 text-xs text-muted-foreground"
        >
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {t("logs.liveUnsupported")}
        </p>
      )}
      {isListBody ? (
        body
      ) : (
        <div className="space-y-2">
          {body}
          {history.loading && history.lines.length === 0 ? null : streamLine}
        </div>
      )}
    </LogThemeFrame>
  );
}

// The range the empty state widens to.
const DAY_RANGE = RANGE_PRESETS.find((preset) => preset.id === "24h")!;

// How long a free-text draft waits before it reaches the queries and the URL,
// so a keystroke doesn't refetch history and reopen the stream.
export const COMMIT_DELAY_MS = 300;
// `filters` is what the controls show; its free-text fields are drafts.
// `committed` is the text and path the queries and the URL carry.
interface LogViewState {
  filters: LogFilters;
  committed: { text: string; path: string; host: string };
}

function viewFromUrl(filters: LogFilters): LogViewState {
  return {
    filters,
    committed: { text: filters.text, path: filters.path, host: filters.host },
  };
}

/** One comparable value for a whole filter state. */
function logStateKey(filters: LogFilters) {
  return JSON.stringify([
    filters.type,
    filters.text,
    ...STRUCTURED_FILTER_KEYS.map((key) => filters[key]),
  ]);
}

function commitDraft(
  state: LogViewState,
  field: "text" | "path" | "host",
  value: string,
): LogViewState {
  if (field === "text") {
    // Render's `status_code:/4../`-style operators become the matching
    // structured filters (chips) and only the remaining words search text
    // (w4/538).
    const parsed = parseSearchOperators(value);
    if (Object.keys(parsed.filters).length > 0) {
      const filters = {
        ...state.filters,
        ...parsed.filters,
        text: parsed.text,
      };
      return {
        ...state,
        filters,
        committed: {
          text: parsed.text,
          path: filters.path,
          host: filters.host,
        },
      };
    }
  }
  return { ...state, committed: { ...state.committed, [field]: value } };
}

// The footer under a Live tail's lines: a pulse while streaming. Every other
// range is a snapshot the picker names, so it has no footer (w3/m537); why a
// tail can't run at all is said under the filter bar instead, beside the
// picker whose Live tail it disables.
export function StreamStatus({ status }: { status: LiveStatus }) {
  const { t } = useTranslations();
  return (
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
      {status === "open" ? (
        <>
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-500 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
          </span>
          {t("logs.streaming")}
        </>
      ) : null}
    </p>
  );
}

/** The banner over a live tail whose stream dropped and is reconnecting. */
export function StreamDisconnected() {
  const { t } = useTranslations();
  return (
    <div className="flex items-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
      <WifiOff className="h-4 w-4" />
      {t("logs.disconnected")}
    </div>
  );
}
