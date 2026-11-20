import { useEffect, useId, useMemo, useState, type ReactNode } from "react";
import {
  AlertCircle,
  Database,
  LockKeyhole,
  ScrollText,
  Search,
} from "lucide-react";
import { PlanBillingLink } from "@/common/components/plan-billing-link";
import { RemovableChip } from "@/common/components/removable-chip";
import { Input } from "@/common/components/ui/input.tsx";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  planFeatureRequired,
  relayableReason,
} from "@/common/lib/graphql-error";
import { cn } from "@/common/lib/utils/utils.ts";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { RangeSelect } from "@/features/metrics/components/range-select";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import {
  isCustomRange,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { useCustomWindow } from "../hooks/use-custom-window";
import {
  useLiveLogs,
  useNewLogsWatch,
  type LogSubscription,
  type EventSourceFactory,
} from "../hooks/use-live-logs";
import { useLogHistory } from "../hooks/use-log-history";
import { useLogRetentionBound } from "../hooks/use-log-retention";
import { parseSearchOperators } from "../lib/log-search-operators";
import { mergeLogLines } from "../lib/map";
import { resourceLabels } from "../lib/resource-labels";
import {
  EMPTY_LOG_FILTERS,
  LOG_PAGE_SIZE,
  LOG_TEXT_MODE_DISPLAY,
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
  STRUCTURED_FILTER_KEYS,
  STRUCTURED_LABEL_KEYS,
  lineFilterKey,
  liveTailUnsupported,
  withoutRequestFilters,
  type LogFilters,
  type LogLine,
  type ProjectLogScope,
  type StructuredFilterKey,
} from "../types";
import type { LogRowMenuItems } from "./log-row-menu";
import { LogClearedState, LogExplorerActions } from "./log-explorer";
import { useClearedLines, useLogExplorer } from "../hooks/use-log-explorer";
import { LiveToggle } from "./log-filter-bar";
import { LogEmptyState } from "./log-empty-state";
import { LogLineList } from "./log-line-list";
import { LogPanelSkeleton } from "./log-panel-skeleton";
import { LogPanelState } from "./log-panel-state";
import {
  LOG_ACTIONS_CLASS,
  LOG_MAXIMIZED_BODY_CLASS,
  LOG_MAXIMIZED_CLASS,
  LOG_MAXIMIZED_LIST_CLASS,
} from "./log-toolbar-layout";
import { LogTruncationNotice } from "./log-truncation-notice";
import {
  COMMIT_DELAY_MS,
  StreamDisconnected,
  StreamStatus,
} from "./log-viewer";

type Picks = Partial<Pick<LogFilters, StructuredFilterKey>>;

export interface ProjectLogViewerProps {
  /** The view's scope; keep it stable (it is the query's identity). */
  scope: ProjectLogScope;
  /** The view's log type: Application Logs read `app`, Request Logs `request`. */
  logType: typeof LOG_TYPE_APP | typeof LOG_TYPE_REQUEST;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
  /** The line a View in context link marks (w3/m528): the view opens on it
   *  with Live off. */
  contextLine?: string;
  /** View in context's link for a line, offered while a search or filter
   *  narrows the view (w3/m528). */
  contextHref?: (line: LogLine) => string | undefined;
  /** Injectable SSE factory — threaded through for tests. */
  createEventSource?: EventSourceFactory;
}

/**
 * A project observability view's log explorer (w3/m505): every service, cron
 * job, Postgres and Key Value the view reads, in one time-ordered stream,
 * each line naming its resource (Render's colored dot and `[name]`). The
 * toolbar is Render's essentials — the search box (with Render's
 * `level:`/`instance:`/… operators), the range and the live tail; the view's
 * own filters travel as its filter document, and bex-api resolves the
 * resources from it on every read and while tailing.
 *
 * Each state reads differently and none claims "no logs" for a read that
 * failed: Request Logs on a plan without them (Render: Pro and higher), the
 * log store unavailable, a timed-out or failed read, a role that reads no
 * logs (Render: Contributor and up view logs, w3/078), a view with nothing to
 * read in it, and a range with no lines.
 */
export function ProjectLogViewer({
  scope,
  logType,
  range,
  onRangeChange,
  contextLine,
  contextHref,
  createEventSource,
}: ProjectLogViewerProps) {
  const { t } = useTranslations();
  const { denied } = useCapabilities();
  const hydrated = useIsHydrated();
  const [draft, setDraft] = useState("");
  const [committed, setCommitted] = useState<{ text: string; picks: Picks }>({
    text: "",
    picks: {},
  });
  // A View in context link opens on its line with the tail paused, as its
  // custom range would on Render.
  const [live, setLive] = useState(!contextLine);

  // Typed text commits after a pause; Render's field operators become chips
  // and only the remaining words search text (w4/538), as on a service's tab.
  useEffect(() => {
    if (draft === committed.text) return;
    const id = setTimeout(() => {
      const parsed = parseSearchOperators(draft);
      setCommitted((previous) => ({
        text: parsed.text,
        picks: { ...previous.picks, ...parsed.filters },
      }));
      if (Object.keys(parsed.filters).length > 0) setDraft(parsed.text);
    }, COMMIT_DELAY_MS);
    return () => clearTimeout(id);
  }, [draft, committed.text]);

  const queryFilters = useMemo<LogFilters>(() => {
    const filters = {
      ...EMPTY_LOG_FILTERS,
      ...committed.picks,
      type: logType,
      text: committed.text,
    };
    // Application Logs carry no request line, so no request filter.
    return logType === LOG_TYPE_APP ? withoutRequestFilters(filters) : filters;
  }, [committed, logType]);
  const picks = STRUCTURED_FILTER_KEYS.filter((key) => queryFilters[key]);

  const { customEndMs, windowClosed } = useCustomWindow(range);
  const filtersLiveSupported = !liveTailUnsupported(queryFilters);
  const liveSupported = filtersLiveSupported && !windowClosed;
  const liveOffReasonId = useId();

  const historyWindow = useLiveRange(
    range,
    JSON.stringify([scope.project, scope.filterDocument, queryFilters]),
    { paused: !live },
  );
  const history = useLogHistory(
    "",
    queryFilters,
    historyWindow,
    JSON.stringify(range),
    LOG_TEXT_MODE_DISPLAY,
    scope,
  );
  const planRefused = planFeatureRequired(history.error, "requestLogs");
  const tailing = live && liveSupported && !planRefused;
  // What the tail follows, and the paused view's watch with it (w3/m528).
  const subscription: LogSubscription = {
    resource: "",
    project: scope,
    type: logType,
    text: committed.text,
    textMode: LOG_TEXT_MODE_DISPLAY,
    instance: queryFilters.instance,
    requestFilters: queryFilters,
  };
  const stream = useLiveLogs({
    ...subscription,
    enabled: tailing,
    readerKey: lineFilterKey(queryFilters),
    startTime: historyWindow.startTime,
    createEventSource,
  });
  const lines = useMemo(
    () =>
      mergeLogLines(
        history.lines,
        stream.lines,
        Date.parse(historyWindow.startTime),
        customEndMs ?? Infinity,
      ),
    [history.lines, stream.lines, historyWindow.startTime, customEndMs],
  );
  const labels = useMemo(
    () => resourceLabels(history.resources),
    [history.resources],
  );
  // Every resource of a project shares its workspace's plan, so any one of
  // them bounds the range as the server floors the read.
  const rangeBound = useLogRetentionBound(history.resources?.[0]?.id ?? "");
  const emptyScope = history.resources?.length === 0;
  // Render's explorer controls (w3/m509): Options, Maximize and the
  // shortcuts, `Shift+L` clearing the lines while the tail is live.
  // The view's identity: what a clear and the paused view's watch reset on.
  const viewKey = JSON.stringify([
    scope.project,
    scope.filterDocument,
    queryFilters,
    range,
  ]);
  const cleared = useClearedLines(lines, tailing, viewKey);
  const { explorer, attachList, attachSearch } = useLogExplorer({
    clear: cleared.clear,
  });
  // Render's row menu (w3/m528): View in context while a search or filter
  // narrows the view. A project view offers no SSH, as Render's
  // (`servicePath: null`).
  const narrowed = picks.length > 0 || committed.text !== "";
  const contextLink = narrowed ? contextHref : undefined;
  const rowMenu = useMemo<LogRowMenuItems>(
    () => ({ contextHref: contextLink }),
    [contextLink],
  );
  // Paused on a preset range, a new line brings up Render's "New logs in
  // Live Tail" (w3/m528).
  const watch = useNewLogsWatch({
    enabled: !live && liveSupported && !planRefused && !isCustomRange(range),
    subscription,
    since: historyWindow.endTime,
    resetKey: viewKey,
    createEventSource,
  });

  const historyReason = relayableReason(history.error);
  // Every state without lines is drawn in the log panel's own screen-filling
  // box, as the loading panel and the lines are, so the view keeps one shape
  // whatever its read answers (as the service Logs tab does).
  let body: ReactNode;
  let listShown = false;
  if (planRefused) {
    body = (
      <LogPanelState
        fillViewport
        icon={LockKeyhole}
        title={t("logs.requestPlanTitle")}
        body={
          <>
            {t("logs.requestPlanBody")} <PlanBillingLink />
          </>
        }
      />
    );
  } else if (history.storeUnavailable && history.lines.length === 0) {
    body = (
      <LogPanelState
        fillViewport
        icon={Database}
        title={t("logs.storeRequiredTitle")}
        body={t("logs.storeRequiredBody")}
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
      />
    );
  } else if (history.loading && history.lines.length === 0) {
    body = <LogPanelSkeleton fillViewport />;
  } else if (cleared.cleared && cleared.lines.length === 0) {
    body = <LogClearedState fillViewport onRestore={cleared.restore} />;
  } else if (emptyScope && lines.length === 0 && denied("can_view_logs")) {
    // bex-api reads a caller without log access an empty scope, never naming
    // a resource (ADR010 § Project views), so the role says why it is empty.
    body = (
      <LogPanelState
        fillViewport
        icon={LockKeyhole}
        title={t("logs.projectRoleLimitTitle")}
        body={t("logs.projectRoleLimitBody")}
      />
    );
  } else if (emptyScope && lines.length === 0) {
    body = (
      <LogPanelState
        fillViewport
        icon={ScrollText}
        title={t("logs.projectScopeEmptyTitle")}
        body={
          logType === LOG_TYPE_REQUEST
            ? t("logs.projectRequestScopeEmptyBody")
            : t("logs.projectScopeEmptyBody")
        }
      />
    );
  } else if (lines.length === 0) {
    body = (
      <LogEmptyState
        reason="empty"
        fillViewport
        filtered={picks.length > 0 || committed.text !== ""}
        emptyTitle={t("logs.projectEmptyTitle")}
        emptyBody={t("logs.projectEmptyBody")}
        pages={history}
      />
    );
  } else {
    listShown = true;
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
        {live && liveSupported && stream.status === "error" ? (
          <StreamDisconnected />
        ) : null}
        <div className={cn(explorer.maximized && LOG_MAXIMIZED_LIST_CLASS)}>
          <LogLineList
            fillViewport
            fill={explorer.maximized}
            lines={cleared.lines}
            resourceLabels={labels}
            hasMore={history.hasMore}
            loadingOlder={history.loadingOlder}
            onLoadOlder={history.loadOlder}
            order={explorer.order}
            expandAll={explorer.expandAll}
            handleRef={attachList}
            holdWhileAway={tailing}
            rowMenu={rowMenu}
            markedKey={contextLine}
            newLogsInLiveTail={
              watch.available
                ? { onSwitch: () => setLive(true), onDismiss: watch.dismiss }
                : undefined
            }
          />
        </div>
        {liveSupported ? (
          <StreamStatus live={live} status={stream.status} />
        ) : null}
      </div>
    );
  }

  return (
    <div className={cn("space-y-4", explorer.maximized && LOG_MAXIMIZED_CLASS)}>
      {/* A phone gets [search] then [range | Live | Maximize | Options];
          the skeleton (ProjectLogViewPageSkeleton) holds the same two
          rows. */}
      <div className="flex flex-wrap items-center gap-3 max-sm:gap-2">
        <div className="relative min-w-48 flex-1 max-sm:min-w-0 max-sm:basis-full">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            ref={attachSearch}
            value={draft}
            // Uneditable until handlers attach: text typed into the server
            // HTML is wiped by hydration (w4/m219).
            disabled={!hydrated}
            onChange={(event) => setDraft(event.target.value)}
            placeholder={t("logs.searchPlaceholder")}
            aria-label={t("logs.searchPlaceholder")}
            className="h-9 pl-8"
          />
        </div>
        <RangeSelect
          range={range}
          onRangeChange={onRangeChange}
          ariaLabel={t("logs.rangeLabel")}
          bound={rangeBound}
          triggerClassName="max-sm:w-auto max-sm:min-w-0 max-sm:flex-1"
        />
        <LiveToggle
          id="project-logs-live-toggle"
          checked={live && liveSupported}
          disabled={!hydrated || !liveSupported}
          onCheckedChange={setLive}
          describedBy={liveSupported ? undefined : liveOffReasonId}
        />
        <LogExplorerActions
          explorer={explorer}
          canCopy={listShown}
          className={LOG_ACTIONS_CLASS}
        />
        {picks.length > 0 ? (
          <div className="flex w-full flex-wrap items-center gap-1.5">
            {picks.map((key) => {
              const label = t(STRUCTURED_LABEL_KEYS[key]);
              return (
                <RemovableChip
                  key={key}
                  removeLabel={t("logs.chipRemove", { label })}
                  onRemove={() =>
                    setCommitted((previous) => ({
                      ...previous,
                      picks: { ...previous.picks, [key]: "" },
                    }))
                  }
                >
                  {label}: {queryFilters[key]}
                </RemovableChip>
              );
            })}
          </div>
        ) : null}
      </div>
      <p
        id={liveSupported ? undefined : liveOffReasonId}
        className="flex items-center gap-1.5 text-xs text-muted-foreground"
      >
        {liveSupported ? (
          t("logs.rangeHistoryNote")
        ) : (
          <>
            <AlertCircle className="h-3.5 w-3.5 shrink-0" />
            {filtersLiveSupported
              ? t("logs.liveWindowClosed")
              : t("logs.liveUnsupported")}
          </>
        )}
      </p>
      {body}
    </div>
  );
}
