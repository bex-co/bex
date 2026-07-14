import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDebounce } from "@/common/hooks/use-debounce";
import { EmptyState } from "@/common/components/empty-state";
import { Input } from "@/common/components/ui/input";
import { LogLineList } from "@/features/logs/components/log-line-list";
import { useDeployLogs } from "../hooks/use-deploy-logs";

export interface DeployLogPanelProps {
  resource: string;
  startTime: string | undefined;
  endTime: string | undefined;
  /** Whether this deploy has a pre-deploy step — skips the predeploy log leg when false. */
  hasPreDeploy: boolean;
}

/**
 * The deploy detail page's log viewer (w9/m1/t003): a deploy-window-scoped
 * twin of the Logs tab's full viewer, minus the type/level/method filter
 * dropdowns the Logs tab needs and this page doesn't — a deploy already knows
 * its own window and always wants build+predeploy+app interleaved. Search is
 * a plain client-side substring filter over the merged lines (the windowed
 * query itself has no `text` arg wired here, since it would triple-fetch on
 * every keystroke across three type queries); follow-to-bottom comes from the
 * reused LogLineList.
 */
export function DeployLogPanel({
  resource,
  startTime,
  endTime,
  hasPreDeploy,
}: DeployLogPanelProps) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search, 300);

  const { lines, loading, error, buildStoreUnavailable } = useDeployLogs(
    resource,
    startTime,
    endTime,
    hasPreDeploy,
  );

  const filtered = useMemo(
    () =>
      debouncedSearch
        ? lines.filter((l) =>
            l.message.toLowerCase().includes(debouncedSearch.toLowerCase()),
          )
        : lines,
    [lines, debouncedSearch],
  );

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
  } else if (filtered.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("logs.emptyTitle")}
        description={
          debouncedSearch ? t("logs.emptyFilteredBody") : t("logs.emptyBody")
        }
      />
    );
  } else {
    body = <LogLineList lines={filtered} />;
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("deploys.logSearchPlaceholder")}
          className="max-w-sm"
        />
        {buildStoreUnavailable && (
          <span className="text-xs text-muted-foreground">
            {t("deploys.buildLogsStoreUnavailable")}
          </span>
        )}
      </div>
      {body}
    </div>
  );
}
