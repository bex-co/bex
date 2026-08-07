import { useMemo, useState } from "react";
import {
  ChevronDown,
  Loader2,
  Maximize2,
  Minimize2,
  Settings2,
  WifiOff,
} from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
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
import { cn } from "@/common/lib/utils/utils.ts";
import { LogLineList } from "@/features/logs/components/log-line-list";
import type { EventSourceFactory } from "@/features/logs/hooks/use-live-logs";
import { useDeployLogs } from "../hooks/use-deploy-logs";
import { LOG_RANGES, RANGE_LABEL_KEYS, type LogRange } from "../lib/log-range";

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

function matchesTypeChoice(lineType: string, choice: LogTypeChoice): boolean {
  if (choice === "all") return true;
  if (choice === "build") return lineType === "build";
  return lineType !== "build";
}

export interface DeployLogPanelProps {
  resource: string;
  startTime: string | undefined;
  endTime: string | undefined;
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
 * button that expands the viewer into a full-screen overlay.
 */
export function DeployLogPanel({
  resource,
  startTime,
  endTime,
  hasPreDeploy,
  followBuild,
  range,
  onRangeChange,
  createEventSource,
}: DeployLogPanelProps) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search, 300);
  const [typeChoice, setTypeChoice] = useState<LogTypeChoice>("all");
  const [instanceFilter, setInstanceFilter] = useState("");
  const [wrap, setWrap] = useState(true);
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [maximized, setMaximized] = useState(false);

  const { lines, loading, error, buildStoreUnavailable, buildLiveStatus } =
    useDeployLogs(
      resource,
      startTime,
      endTime,
      hasPreDeploy,
      followBuild,
      createEventSource,
    );

  const filtered = useMemo(() => {
    let out =
      typeChoice === "all"
        ? lines
        : lines.filter((l) => matchesTypeChoice(l.type, typeChoice));
    if (debouncedSearch) {
      out = out.filter((l) =>
        l.message.toLowerCase().includes(debouncedSearch.toLowerCase()),
      );
    }
    if (instanceFilter) {
      out = out.filter((l) => l.instance === instanceFilter);
    }
    return out;
  }, [lines, debouncedSearch, typeChoice, instanceFilter]);

  const narrowed =
    !!debouncedSearch || typeChoice !== "all" || !!instanceFilter;

  let body;
  if (error) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("logs.errorTitle")}
        description={error.message}
      />
    );
  } else if (loading && lines.length === 0) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t("logs.loading")}
      </div>
    );
  } else if (narrowed && lines.length > 0 && filtered.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("logs.emptyTitle")}
        description={t("logs.emptyFilteredBody")}
      />
    );
  } else if (buildStoreUnavailable && lines.length === 0) {
    body = (
      <EmptyState
        iconName="DatabaseZap"
        title={t("deploys.buildLogsStoreUnavailable")}
        description={t("deploys.buildLogsStoreUnavailableBody")}
      />
    );
  } else if (filtered.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("logs.emptyTitle")}
        description={t("logs.emptyBody")}
      />
    );
  } else {
    body = (
      <LogLineList
        lines={filtered}
        onInstanceFilter={setInstanceFilter}
        wrap={wrap}
        showTimestamps={showTimestamps}
        fill={maximized}
      />
    );
  }

  return (
    <div
      className={cn(
        "space-y-3",
        // Maximized (w9/003): a full-screen overlay, the log list filling
        // everything under the control row.
        maximized &&
          "fixed inset-0 z-50 flex flex-col bg-background p-4 sm:p-6 [&>*:last-child]:min-h-0 [&>*:last-child]:flex-1",
      )}
    >
      <div className="flex items-center gap-2">
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
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("deploys.logSearchPlaceholder")}
          className="max-w-sm"
        />
        {followBuild && buildLiveStatus === "error" && (
          <span className="flex items-center gap-1 text-xs text-muted-foreground">
            <WifiOff className="h-3.5 w-3.5" />
            {t("logs.disconnected")}
          </span>
        )}
        <div className="ml-auto flex shrink-0 items-center gap-1">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                aria-label={t("deploys.logOptions")}
                title={t("deploys.logOptions")}
              >
                <Settings2 className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
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
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            variant="outline"
            size="icon"
            aria-label={
              maximized ? t("deploys.logMinimize") : t("deploys.logMaximize")
            }
            title={
              maximized ? t("deploys.logMinimize") : t("deploys.logMaximize")
            }
            onClick={() => setMaximized((m) => !m)}
          >
            {maximized ? (
              <Minimize2 className="h-4 w-4" />
            ) : (
              <Maximize2 className="h-4 w-4" />
            )}
          </Button>
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
      {body}
    </div>
  );
}
