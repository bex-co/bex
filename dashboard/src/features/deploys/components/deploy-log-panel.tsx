import { useMemo, useState } from "react";
import { ChevronDown, WifiOff } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useDebounce } from "@/common/hooks/use-debounce";
import { EmptyState } from "@/common/components/empty-state";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
import { RemovableChip } from "@/common/components/removable-chip";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { LogEmptyState } from "@/features/logs/components/log-empty-state";
import {
  LogOptionsMenu,
  MaximizeButton,
} from "@/features/logs/components/log-explorer";
import { useLogExplorer } from "@/features/logs/hooks/use-log-explorer";
import { LogLineList } from "@/features/logs/components/log-line-list";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { LogTruncationNotice } from "@/features/logs/components/log-truncation-notice";
import { stripAnsi } from "@/features/logs/lib/ansi";
import { goLowercase } from "@/features/logs/lib/go-lowercase";
import type { EventSourceFactory } from "@/features/logs/hooks/use-live-logs";
import { useDeployLogs } from "../hooks/use-deploy-logs";
import {
  DEPLOY_LOG_TOOLBAR_CLASS,
  DEPLOY_LOG_TOOLBAR_END_CLASS,
} from "./deploy-page-layout";
import { LOG_RANGES, RANGE_LABEL_KEYS, type LogRange } from "../lib/log-range";
import { LogThemeFrame } from "@/features/logs/components/log-theme-frame";

// The radio group needs a value for "no ?r= param" (the deploy window).
const DEPLOY_WINDOW = "deploy";

// Render's deploy-page type selector (w1/029): All logs / Application logs /
// Build logs — a client-side filter over the already-merged lines, so the
// three windowed queries stay untouched. "app" covers app + pre-deploy lines
// (Render's Application bucket has no separate pre-deploy entry).
type LogTypeChoice = "all" | "app" | "build";

const LOG_TYPE_KEYS: Record<LogTypeChoice, string> = {
  all: "deploys.logTypeAll",
  app: "deploys.logTypeApp",
  build: "deploys.logTypeBuild",
};

export interface DeployLogPanelProps {
  resource: string;
  /** The deploy's name, shown over the log in full-screen mode. */
  title?: string;
  /** The deploy whose page this is: its build and pre-deploy lines are scoped
   *  to it, not just to its time window (w21/m324). A relative `range` widens
   *  the view to the service's logs across deploys. */
  deployId?: string;
  startTime: string | undefined;
  endTime: string | undefined;
  /** When the deploy began (its creation, whatever range is shown): each
   *  line's time card says how long after it the line came, Render's From
   *  start (w4/716). */
  startedAt?: string;
  /** Whether this deploy has a pre-deploy step — skips the predeploy log leg when false. */
  hasPreDeploy: boolean;
  /** Follow the active build pod while this deploy is build_in_progress. */
  followBuild: boolean;
  /** The active `?r=` relative window (w9/003); undefined = the deploy window. */
  range?: LogRange;
  /** Selects a relative window (undefined = back to the deploy window).
   *  Omitted => the range picker is hidden (no URL to write it to). */
  onRangeChange?: (range: LogRange | undefined) => void;
  /** Test seam for the build SSE source. */
  createEventSource?: EventSourceFactory;
  /**
   * Set when the deploy window may end before a canceled deploy's continued
   * pre-deploy step did (w4/m198): its end fact is missing, or the read failed
   * (`onRetry` re-reads it). The wider ranges stay available in Log options.
   */
  limitedWindow?: { onRetry?: () => void };
}

/**
 * The deploy detail page's log viewer (w9/m1/t003): a deploy-window-scoped
 * twin of the Logs tab's full viewer, minus the level/method filter dropdowns
 * the Logs tab needs and this page doesn't — a deploy already knows its own
 * window and defaults to build+predeploy+app interleaved. Render's deploy
 * explorer offers exactly a type selector (All / Application / Build logs) on
 * top of that default, so this panel carries the same three-way choice as a
 * client-side filter over the merged lines (w1/029). Search is likewise a
 * plain client-side substring filter (the windowed query itself has no `text`
 * arg wired here, since it would triple-fetch on every keystroke across three
 * type queries); follow-to-bottom comes from the reused LogLineList.
 *
 * w9/003 adds Render's viewer affordances: an options menu (the `?r=` time
 * range the URL owns, plus wrap/timestamp display toggles) and a maximize
 * button that expands the viewer into a full-screen overlay. w3/m509 gives
 * it every explorer's Options (Copy logs, Order by, Display, w3/088) and
 * Render's shortcuts (`/`, `M`, `Esc`, `Home`/`End`, `⌘/Ctrl ⇧ C`); it has
 * no live tail of its own, so no Clear logs or `Shift+L` (w3/092).
 */
export function DeployLogPanel({
  resource,
  title,
  deployId,
  startTime,
  endTime,
  startedAt,
  hasPreDeploy,
  followBuild,
  range,
  onRangeChange,
  createEventSource,
  limitedWindow,
}: DeployLogPanelProps) {
  const { t } = useTranslations();
  const hydrated = useIsHydrated();
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search, 300);
  const [typeChoice, setTypeChoice] = useState<LogTypeChoice>("all");
  const [instanceFilter, setInstanceFilter] = useState("");
  const [wrap, setWrap] = useState(true);
  const [showTimestamps, setShowTimestamps] = useState(true);
  const { explorer, attachList, attachSearch } = useLogExplorer();
  const { maximized } = explorer;

  const {
    lines,
    inLogBucket,
    loading,
    error,
    buildStoreUnavailable,
    buildLiveStatus,
    hasMore,
    loadingOlder,
    loadOlder,
  } = useDeployLogs(
    resource,
    startTime,
    endTime,
    hasPreDeploy,
    followBuild,
    createEventSource,
    // A picked range asks for the service's logs across deploys.
    range ? undefined : deployId,
  );

  const filtered = useMemo(() => {
    let out =
      typeChoice === "all"
        ? lines
        : lines.filter((l) => inLogBucket(l, typeChoice));
    if (debouncedSearch) {
      // Match the *displayed* text, not the wire bytes: a build line's color
      // escapes sit between words, so a raw substring search silently misses
      // any needle that straddles one. Case folds the way the server's log
      // search does (Go's per-rune lowercase), so the same term finds the
      // same lines here as in service Logs (w4/m216).
      const needle = goLowercase(debouncedSearch);
      out = out.filter((l) =>
        goLowercase(stripAnsi(l.message)).includes(needle),
      );
    }
    if (instanceFilter) {
      out = out.filter((l) => l.instance === instanceFilter);
    }
    return out;
  }, [lines, inLogBucket, debouncedSearch, typeChoice, instanceFilter]);

  const narrowed =
    !!debouncedSearch || typeChoice !== "all" || !!instanceFilter;

  let body;
  if (error) {
    body = (
      <LogEmptyState
        reason="failed"
        title={t("logs.errorTitle")}
        error={error}
      />
    );
  } else if ((loading || startTime === undefined) && lines.length === 0) {
    // `startTime === undefined` = still awaiting the deploy's window (the panel
    // mounts in parallel with the header query, w9/m62 t002); show the same
    // loading affordance rather than a premature "no logs" empty state.
    body = <LogPanelSkeleton fillViewport="flush" />;
  } else if (buildStoreUnavailable && lines.length === 0) {
    body = (
      <EmptyState
        iconName="DatabaseZap"
        title={t("deploys.buildLogsStoreUnavailable")}
        description={t("deploys.buildLogsStoreUnavailableBody")}
      />
    );
  } else if (filtered.length === 0) {
    // The panel shows its own truncation notice, so no pages are passed.
    body = (
      <LogEmptyState
        reason="empty"
        filtered={narrowed && lines.length > 0}
        emptyTitle={t("logs.emptyTitle")}
        emptyBody={t("logs.emptyBody")}
      />
    );
  } else {
    body = (
      <LogLineList
        lines={filtered}
        onInstanceFilter={setInstanceFilter}
        wrap={wrap}
        showTimestamps={showTimestamps}
        order={explorer.order}
        display={explorer.display}
        handleRef={attachList}
        fill={maximized}
        // The deploy's log runs to the bottom of the screen, as the Logs
        // tab's does, and anything under it starts at the fold rather than
        // being cut by it. It keeps that height however few its lines (the
        // loading placeholder holds the same box, so nothing moves when they
        // land); a finished deploy's log says where it ends, and one that
        // only just overflows grows to show whole rather than hide a row.
        fillViewport="flush"
        ended={endTime !== undefined}
        hasMore={hasMore}
        loadingOlder={loadingOlder}
        onLoadOlder={loadOlder}
        startedAt={startedAt}
      />
    );
  }

  return (
    // Maximized (w9/003): a full-screen overlay, the log list filling
    // everything under the control row.
    <LogThemeFrame className="space-y-3" maximized={maximized}>
      {/* Full screen covers the page's own title, so it names the log. */}
      {maximized && title ? (
        <p className="truncate text-sm font-medium">{title}</p>
      ) : null}
      <div className={DEPLOY_LOG_TOOLBAR_CLASS}>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="outline"
              className="shrink-0"
              aria-label={t("deploys.logTypeFilter")}
            >
              {t(LOG_TYPE_KEYS[typeChoice] as Parameters<typeof t>[0])}
              <ChevronDown className="ml-1 h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuRadioGroup
              value={typeChoice}
              onValueChange={(v) => setTypeChoice(v as LogTypeChoice)}
            >
              {(Object.keys(LOG_TYPE_KEYS) as LogTypeChoice[]).map((choice) => (
                <DropdownMenuRadioItem key={choice} value={choice}>
                  {t(LOG_TYPE_KEYS[choice] as Parameters<typeof t>[0])}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        <Input
          ref={attachSearch}
          value={search}
          // Uneditable until handlers attach: text typed into the server HTML
          // is wiped by hydration (w4/m219).
          disabled={!hydrated}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("deploys.logSearchPlaceholder")}
          // Search takes the row's spare width, as Render's does.
          className="min-w-0 flex-1"
        />
        {followBuild && buildLiveStatus === "error" && (
          <span className="flex items-center gap-1 text-xs text-muted-foreground">
            <WifiOff className="h-3.5 w-3.5" />
            {t("logs.disconnected")}
          </span>
        )}
        <div className={DEPLOY_LOG_TOOLBAR_END_CLASS}>
          <LogOptionsMenu
            explorer={explorer}
            canCopy={filtered.length > 0}
            triggerClassName="size-9"
          >
            {onRangeChange && (
              <>
                <DropdownMenuLabel>
                  {t("deploys.logRangeLabel")}
                </DropdownMenuLabel>
                <DropdownMenuRadioGroup
                  value={range ?? DEPLOY_WINDOW}
                  onValueChange={(v) =>
                    onRangeChange(
                      v === DEPLOY_WINDOW ? undefined : (v as LogRange),
                    )
                  }
                >
                  <DropdownMenuRadioItem value={DEPLOY_WINDOW}>
                    {t("deploys.logRangeDeploy")}
                  </DropdownMenuRadioItem>
                  {LOG_RANGES.map((r) => (
                    <DropdownMenuRadioItem key={r} value={r}>
                      {t(RANGE_LABEL_KEYS[r] as Parameters<typeof t>[0])}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
                <DropdownMenuSeparator />
              </>
            )}
            <DropdownMenuCheckboxItem
              checked={wrap}
              onCheckedChange={(v) => setWrap(!!v)}
            >
              {t("deploys.logWrap")}
            </DropdownMenuCheckboxItem>
            <DropdownMenuCheckboxItem
              checked={showTimestamps}
              onCheckedChange={(v) => setShowTimestamps(!!v)}
            >
              {t("deploys.logTimestamps")}
            </DropdownMenuCheckboxItem>
          </LogOptionsMenu>
          <MaximizeButton explorer={explorer} className="size-9" />
        </div>
      </div>
      {instanceFilter ? (
        <div className="flex flex-wrap items-center gap-1.5">
          <RemovableChip
            removeLabel={t("logs.chipRemove", {
              label: t("logs.instanceLabel"),
            })}
            onRemove={() => setInstanceFilter("")}
          >
            {t("logs.instanceLabel")}: {instanceFilter}
          </RemovableChip>
        </div>
      ) : null}
      {buildStoreUnavailable && lines.length > 0 ? (
        <div
          role="status"
          className="rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground"
        >
          <span className="font-medium text-foreground">
            {t("deploys.buildLogsStoreUnavailable")}.
          </span>{" "}
          {t("deploys.buildLogsStoreUnavailableBody")}
        </div>
      ) : null}
      {limitedWindow && !range ? (
        <div
          role="status"
          className="flex flex-wrap items-center gap-2 rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground"
        >
          {limitedWindow.onRetry ? (
            <>
              <span>{t("deploys.logStepEndUnavailable")}</span>
              <Button
                variant="outline"
                size="sm"
                className="h-6 px-2 text-xs"
                onClick={limitedWindow.onRetry}
              >
                {t("common.tryAgain")}
              </Button>
            </>
          ) : (
            <span>{t("deploys.logStepEndMissing")}</span>
          )}
        </div>
      ) : null}
      {hasMore && lines.length > 0 ? (
        <LogTruncationNotice
          loadingOlder={loadingOlder}
          onLoadOlder={loadOlder}
          message={t("deploys.logPartiallyLoaded")}
        />
      ) : null}
      {body}
    </LogThemeFrame>
  );
}
