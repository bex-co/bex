import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { AlertCircle, Loader2, WifiOff } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDebounce } from "@/common/hooks/use-debounce";
import { EmptyState } from "@/common/components/empty-state";
import { useLogHistory } from "../hooks/use-log-history";
import {
  useLiveLogs,
  type EventSourceFactory,
  type LiveStatus,
} from "../hooks/use-live-logs";
import { mergeLogLines } from "../lib/map";
import {
  EMPTY_LOG_FILTERS,
  LOG_TYPE_ALL,
  STRUCTURED_FILTER_KEYS,
  usesStoreOnlyFilters,
  type LogFilters,
} from "../types";
import { LogFilterBar } from "./log-filter-bar";
import { LogLineList } from "./log-line-list";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import {
  DEFAULT_RANGE_PRESET,
  type RangePreset,
} from "@/features/metrics/lib/range";

interface LogViewerProps {
  resource: string;
  /** Injectable SSE factory — threaded through for tests. */
  createEventSource?: EventSourceFactory;
  range?: RangePreset;
  onRangeChange?: (range: RangePreset) => void;
  /**
   * Initial filter state (from the URL, w7/m42); read ONCE at mount — a later
   * search-only navigation to an already-mounted viewer is ignored (URL→state
   * back/forward sync is deferred with that feature).
   */
  initialFilters?: LogFilters;
  initialLive?: boolean;
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
  range = DEFAULT_RANGE_PRESET,
  onRangeChange = () => undefined,
  initialFilters,
  initialLive,
  onFiltersChange,
}: LogViewerProps) {
  const { t } = useTranslations();
  const [filters, setFilters] = useState<LogFilters>(
    initialFilters ?? EMPTY_LOG_FILTERS,
  );
  const [live, setLive] = useState(initialLive ?? true);

  // Debounce the free-text filters so a keystroke doesn't refetch history and
  // reopen the stream on every character. Scalar deps (not the filters object,
  // whose identity churns per keystroke) keep queryFilters' identity stable
  // while typing, so the URL-sync effect below fires only on settled values.
  const debouncedText = useDebounce(filters.text, 300);
  const debouncedPath = useDebounce(filters.path, 300);
  const queryFilters = useMemo<LogFilters>(
    () => ({
      type: filters.type,
      level: filters.level,
      method: filters.method,
      statusCode: filters.statusCode,
      instance: filters.instance,
      text: debouncedText,
      path: debouncedPath,
    }),
    [
      filters.type,
      filters.level,
      filters.method,
      filters.statusCode,
      filters.instance,
      debouncedText,
      debouncedPath,
    ],
  );

  // Mirror the debounced state into the URL (w7/m42). The first run is
  // skipped — the mount state was just derived from the URL, and navigating
  // during hydration triggers a full document reload loop under TanStack
  // Start (observed live). The ref keeps the route's inline callback out of
  // the sync effect's deps so only real filter/live changes fire; it is
  // refreshed in its own every-render effect (declared first, so it has run
  // by the time the sync effect fires in the same flush).
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
  }, [queryFilters, live]);

  // The live tail reads pod stdout, so it can't serve request logs or the
  // store-only filters — offer live only when none is active (instance is fine,
  // it's a pod name). A store-only filter with live on just pauses the tail.
  const liveSupported = !usesStoreOnlyFilters(queryFilters);

  const historyWindow = useLiveRange(range);
  const history = useLogHistory(resource, queryFilters, historyWindow);
  const stream = useLiveLogs({
    resource,
    enabled: live && liveSupported,
    type: queryFilters.type,
    text: debouncedText,
    instance: queryFilters.instance,
    createEventSource,
  });

  // Live lines append after the historical page, deduped against it.
  const lines = useMemo(
    () => mergeLogLines(history.lines, stream.lines),
    [history.lines, stream.lines],
  );

  const filtered =
    queryFilters.type !== LOG_TYPE_ALL || hasFieldFilter(queryFilters);

  // One body per state (store-unavailable / error / loading-first / empty /
  // list) — resolved to a single node so the render stays flat.
  let body: ReactNode;
  if (history.storeUnavailable) {
    // Request logs / structured filters need the durable store, which isn't
    // wired here (local dev). An explanatory state, not a generic error.
    body = (
      <EmptyState
        iconName="Database"
        title={t("logs.storeRequiredTitle")}
        description={t("logs.storeRequiredBody")}
      />
    );
  } else if (history.error) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("logs.errorTitle")}
        description={history.error.message}
      />
    );
  } else if (history.loading && history.lines.length === 0) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t("logs.loading")}
      </div>
    );
  } else if (lines.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("logs.emptyTitle")}
        description={
          filtered ? t("logs.emptyFilteredBody") : t("logs.emptyBody")
        }
      />
    );
  } else {
    body = (
      <div className="space-y-2">
        {live && liveSupported && stream.status === "error" ? (
          <div className="flex items-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
            <WifiOff className="h-4 w-4" />
            {t("logs.disconnected")}
          </div>
        ) : null}
        <LogLineList
          lines={lines}
          onInstanceFilter={(instance) =>
            setFilters((previous) => ({ ...previous, instance }))
          }
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
        onChange={(patch) => setFilters((prev) => ({ ...prev, ...patch }))}
        live={live}
        onLiveChange={setLive}
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

// Whether any structured/text field filter (beyond the type dropdown) is set —
// drives the "no logs match this filter" vs "no logs yet" empty-state copy.
function hasFieldFilter(f: LogFilters): boolean {
  return f.text !== "" || STRUCTURED_FILTER_KEYS.some((key) => f[key] !== "");
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
