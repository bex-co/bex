import { useMemo, useState } from "react";
import { WifiOff } from "lucide-react";
import { EmptyState } from "@/common/components/empty-state";
import {
  DropdownMenuCheckboxItem,
} from "@/common/components/ui/dropdown-menu";
import { Input } from "@/common/components/ui/input";
import { useDebounce } from "@/common/hooks/use-debounce";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DEPLOY_LOG_TOOLBAR_CLASS,
  DEPLOY_LOG_TOOLBAR_END_CLASS,
} from "@/features/deploys/components/deploy-page-layout";
import { LogEmptyState } from "@/features/logs/components/log-empty-state";
import {
  LogOptionsMenu,
  MaximizeButton,
} from "@/features/logs/components/log-explorer";
import { LogLineList } from "@/features/logs/components/log-line-list";
import { LogPanelSkeleton } from "@/features/logs/components/log-panel-skeleton";
import { LogThemeFrame } from "@/features/logs/components/log-theme-frame";
import { LogTruncationNotice } from "@/features/logs/components/log-truncation-notice";
import type { EventSourceFactory } from "@/features/logs/hooks/use-live-logs";
import { useLogExplorer } from "@/features/logs/hooks/use-log-explorer";
import { stripAnsi } from "@/features/logs/lib/ansi";
import { goLowercase } from "@/features/logs/lib/go-lowercase";
import { useBuildLogs } from "@/features/build-sources/hooks/use-build-logs";

export interface BuildLogPanelProps {
  buildSourceId: string;
  buildId: string;
  title?: string;
  startTime: string | undefined;
  endTime: string | undefined;
  startedAt?: string;
  followBuild: boolean;
  createEventSource?: EventSourceFactory;
}

/**
 * Historical + live SSE build-log viewer scoped strictly to a Build Source
 * (`resource=bds-…`) and build (`build=[bld-…]`, `w2/m611`, `w2/m614/t004`).
 */
export function BuildLogPanel({
  buildSourceId,
  buildId,
  title,
  startTime,
  endTime,
  startedAt,
  followBuild,
  createEventSource,
}: BuildLogPanelProps) {
  const { t } = useTranslations();
  const hydrated = useIsHydrated();
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search, 300);
  const [wrap, setWrap] = useState(true);
  const [showTimestamps, setShowTimestamps] = useState(true);
  const { explorer, attachList, attachSearch } = useLogExplorer();
  const { maximized } = explorer;

  const {
    lines,
    loading,
    error,
    buildStoreUnavailable,
    buildLiveStatus,
    hasMore,
    loadingOlder,
    loadOlder,
  } = useBuildLogs({
    buildSourceId,
    buildId,
    startTime,
    endTime,
    followBuild,
    createEventSource,
  });

  const filtered = useMemo(() => {
    if (!debouncedSearch) return lines;
    const needle = goLowercase(debouncedSearch);
    return lines.filter((l) =>
      goLowercase(stripAnsi(l.message)).includes(needle),
    );
  }, [lines, debouncedSearch]);

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
    body = (
      <LogEmptyState
        reason="empty"
        filtered={Boolean(debouncedSearch) && lines.length > 0}
        emptyTitle={t("buildSources.buildLogsEmptyTitle")}
        emptyBody={t("buildSources.buildLogsEmptyBody")}
      />
    );
  } else {
    body = (
      <LogLineList
        lines={filtered}
        wrap={wrap}
        showTimestamps={showTimestamps}
        order={explorer.order}
        display={explorer.display}
        handleRef={attachList}
        fill={maximized}
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
    <LogThemeFrame
      className="space-y-3"
      maximized={maximized}
      data-region="build-logs"
    >
      {maximized && title ? (
        <p className="truncate text-sm font-medium">{title}</p>
      ) : null}
      <div className={DEPLOY_LOG_TOOLBAR_CLASS}>
        <Input
          ref={attachSearch}
          value={search}
          disabled={!hydrated}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("deploys.logSearchPlaceholder")}
          aria-label={t("buildSources.buildLogsSearchLabel")}
          className="min-w-0 flex-1"
        />
        {followBuild && buildLiveStatus === "error" ? (
          <span className="text-muted-foreground flex items-center gap-1 text-xs">
            <WifiOff className="h-3.5 w-3.5" aria-hidden="true" />
            {t("logs.disconnected")}
          </span>
        ) : null}
        <div className={DEPLOY_LOG_TOOLBAR_END_CLASS}>
          <LogOptionsMenu
            explorer={explorer}
            canCopy={filtered.length > 0}
            triggerClassName="size-9"
          >
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

      {buildStoreUnavailable && lines.length > 0 ? (
        <div
          role="status"
          className="text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs"
        >
          <span className="text-foreground font-medium">
            {t("deploys.buildLogsStoreUnavailable")}.
          </span>{" "}
          {t("deploys.buildLogsStoreUnavailableBody")}
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
