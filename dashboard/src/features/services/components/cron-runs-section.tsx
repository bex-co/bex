import { useEffect, useEffectEvent, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Loader2, Search, TriangleAlert, X } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { InstantTooltip } from "@/common/components/instant-tooltip";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useNow } from "@/common/hooks/use-now";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatDuration, formatTimeAgo } from "@/common/lib/format";
import { STRETCHED_LINK_CLASS } from "@/common/lib/stretched-link";
import { cn } from "@/common/lib/utils/utils";
import { StatusKindIcon } from "@/features/deploys/components/deploy-status-icon";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { ActivityGate } from "@/features/capabilities/components/activity-gate";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import { projectionReason } from "@/features/capabilities/lib/resource-actions";
import { CronTriggerRunButton } from "@/features/services/components/cron-header-actions";
import {
  CRON_RUN_ICON_CELL,
  CRON_RUN_LINE,
  CRON_RUN_MAIN_CELL,
  CRON_RUN_ROW_CLASS,
  CRON_RUNS_COUNT_CLASS,
  CRON_RUNS_SEARCH_CLASS,
  CronRunRowsFrame,
  CronRunsFrame,
  CronRunsListHeader,
  cronRunRowGrid,
} from "@/features/services/components/cron-runs-frame";
import { useCronRuns } from "@/features/services/hooks/use-cron-runs";
import { useServer } from "@/features/services/hooks/use-server";
import {
  CRON_RUN_STATUS_WORD,
  type CronRunStatus,
  cronRunElapsedMs,
  cronRunIconKind,
  cronRunLogsSearch,
  cronRunMatchesSearch,
  cronRunStatus,
  latestSuccessfulRunId,
} from "@/features/services/lib/cron-run-list";
import { hasServed } from "@/features/services/lib/status";
import type { CronRunView } from "@/features/services/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

type Translate = ReturnType<typeof useTranslations>["t"];

// A run's status as Render's tooltip names it.
const STATUS_LABEL = {
  successful: "services.cronRunStatusSucceeded",
  unsuccessful: "services.cronRunStatusFailed",
  canceled: "services.cronRunStatusCanceled",
  pending: "services.cronRunStatusInProgress",
} as const satisfies Record<CronRunStatus, string>;

function statusLabel(
  status: CronRunStatus,
  latestSuccessful: boolean,
  t: Translate,
): string {
  return latestSuccessful
    ? t("services.cronRunStatusLastSuccessful")
    : t(STATUS_LABEL[status]);
}

// Render names a run Manual when a user started it, else Scheduled.
function triggerLabel(run: CronRunView, t: Translate): string {
  return t(
    run.triggeredBy
      ? "services.cronRunTriggerManual"
      : "services.cronRunTriggerScheduled",
  );
}

/**
 * A cron job's Runs page as Render's (w3/m556): "Search cron job runs" over
 * the loaded runs, then RUN with the count, TRIGGER and DURATION. Each row is
 * the run's status icon, "Cron job run", when it ran and who started it, its
 * trigger and its duration, and the whole row opens the Logs over the run's
 * window. A run in progress has Cancel, at once as on Render. With no runs the
 * page says why in Render's words and offers Trigger Run; Load more reads
 * older runs, which bex keeps past Render's 30.
 *
 * Render shows a cron job's runs as service events, so they sit behind the
 * role's service-activity gate (w9/m621).
 */
export function CronRunsSection({ serviceId }: { serviceId: string }) {
  return (
    <ActivityGate>
      <CronRuns serviceId={serviceId} />
    </ActivityGate>
  );
}

function CronRuns({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const hydrated = useIsHydrated();
  const {
    runs,
    loading,
    error,
    loadingMore,
    hasMore,
    cancelingId,
    loadMore,
    cancel,
  } = useCronRuns(serviceId);
  // The layout already holds the service: its build decides which empty
  // state shows, and the empty states carry its Trigger Run.
  const { service } = useServer(serviceId, { poll: false });
  const [query, setQuery] = useState("");
  const cancelBlocked = useCancelBlocked(serviceId, runs);
  const latestSuccessfulId = latestSuccessfulRunId(runs);
  const visibleRuns = useMemo(
    () =>
      runs.filter((run) => {
        const status = cronRunStatus(run);
        const latest = run.id === latestSuccessfulId;
        return cronRunMatchesSearch(
          run,
          [
            t(STATUS_LABEL[status]),
            CRON_RUN_STATUS_WORD[status],
            latest ? t("services.cronRunStatusLastSuccessful") : null,
            triggerLabel(run, t),
            run.triggeredBy,
          ],
          query,
        );
      }),
    [runs, latestSuccessfulId, query, t],
  );

  if (loading && runs.length === 0) {
    return (
      <CronRunsFrame
        search={<CronRunsSearch value="" onChange={() => {}} disabled />}
      >
        <CronRunRowsFrame />
      </CronRunsFrame>
    );
  }
  if (error && runs.length === 0) {
    // Render's load failure stands alone, with no search to type into.
    return (
      <div
        role="alert"
        className="flex items-start gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm"
      >
        <TriangleAlert
          aria-hidden="true"
          className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400"
        />
        <p>{t("services.cronRunsLoadFailed")}</p>
      </div>
    );
  }

  let body;
  if (runs.length === 0) {
    // Render's two empty states: a cron job whose build never succeeded has
    // never run; one with a build has not run lately.
    const built = !service || hasServed(service);
    body = (
      <CronRunsEmpty
        title={t(
          built
            ? "services.cronRunsEmptyRecentTitle"
            : "services.cronRunsEmptyNeverTitle",
        )}
        body={t(
          built
            ? "services.cronRunsEmptyRecentBody"
            : "services.cronRunsEmptyNeverBody",
        )}
        action={service ? <CronTriggerRunButton service={service} /> : null}
      />
    );
  } else if (visibleRuns.length === 0) {
    body = (
      <CronRunsEmpty
        title={t("services.cronRunsNoMatchTitle")}
        body={t("services.cronRunsNoMatchBody")}
        action={
          <Button
            variant="outline"
            size="sm"
            className="max-lg:h-11"
            onClick={() => setQuery("")}
          >
            {t("services.cronRunsResetSearch")}
          </Button>
        }
      />
    );
  } else {
    body = (
      <CronRunList
        serviceId={serviceId}
        runs={visibleRuns}
        latestSuccessfulId={latestSuccessfulId}
        cancelBlocked={cancelBlocked}
        cancelingId={cancelingId}
        onCancel={(run) => void cancel(run.id)}
      />
    );
  }

  return (
    <CronRunsFrame
      search={
        <CronRunsSearch
          value={query}
          onChange={setQuery}
          // Uneditable until handlers attach: a query typed into the server
          // HTML is wiped by hydration (w4/m219).
          disabled={!hydrated}
        />
      }
    >
      {body}
      {hasMore && runs.length > 0 ? (
        <div className="mt-4 flex justify-center">
          <Button
            variant="outline"
            size="sm"
            className="max-lg:h-11"
            disabled={loadingMore}
            onClick={() => void loadMore()}
          >
            {loadingMore
              ? t("services.cronRunsLoadingMore")
              : t("services.cronRunsLoadMore")}
          </Button>
        </div>
      ) : null}
    </CronRunsFrame>
  );
}

/**
 * Why Cancel is unavailable, from the server's own decision (w4/203). The
 * action snapshot is a separate no-cache read that history polling never
 * replaces, so a run arriving after it kept a settled "no active run" Cancel
 * (w4/249). It is re-read whenever the set of active runs changes, and Cancel
 * stays checking until that re-read settles.
 */
function useCancelBlocked(serviceId: string, runs: CronRunView[]) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(serviceId);
  const activeRunKey = runs
    .filter((run) => cronRunStatus(run) === "pending")
    .map((run) => run.id)
    .join(" ");
  const [checkedRunKey, setCheckedRunKey] = useState(activeRunKey);
  const refreshActions = useEffectEvent(() => serverActions.refresh());
  useEffect(() => {
    if (activeRunKey === checkedRunKey) return;
    let live = true;
    void refreshActions().finally(() => {
      if (live) setCheckedRunKey(activeRunKey);
    });
    return () => {
      live = false;
    };
  }, [activeRunKey, checkedRunKey]);
  return projectionReason(
    activeRunKey === checkedRunKey ? serverActions : { status: "checking" },
    currentWorkspaceId,
    serviceId,
    "cron_cancel_run",
    t,
  );
}

/** "Search cron job runs", with Render's clear button once it has text. */
function CronRunsSearch({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  return (
    <>
      <Search
        aria-hidden="true"
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        type="search"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t("services.cronRunsSearchPlaceholder")}
        aria-label={t("services.cronRunsSearchPlaceholder")}
        className={cn(
          "truncate pr-10 pl-9 [&::-webkit-search-cancel-button]:hidden",
          CRON_RUNS_SEARCH_CLASS,
        )}
      />
      {value ? (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("services.cronRunsSearchClear")}
          className="absolute top-1/2 right-1 size-8 -translate-y-1/2 pointer-coarse:size-10"
          onClick={() => onChange("")}
        >
          <X />
        </Button>
      ) : null}
    </>
  );
}

/** Render's empty state: a title, a line on what to do, and its action. */
function CronRunsEmpty({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center gap-1 rounded-lg border border-dashed px-4 py-10 text-center">
      <h3 className="font-medium text-balance">{title}</h3>
      <p className="max-w-md text-sm text-balance text-muted-foreground">
        {body}
      </p>
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}

function CronRunList({
  serviceId,
  runs,
  latestSuccessfulId,
  cancelBlocked,
  cancelingId,
  onCancel,
}: {
  serviceId: string;
  runs: CronRunView[];
  latestSuccessfulId: string | null;
  cancelBlocked: React.ReactNode;
  cancelingId: string | null;
  onCancel: (run: CronRunView) => void;
}) {
  const { t, i18n } = useTranslations();
  const viewerEmail = useViewerEmail();
  // One clock for the list: by the second while a run's duration counts up,
  // else by the minute for the rows' ages.
  const running = runs.some(
    (run) => cronRunStatus(run) === "pending" && run.startedAt,
  );
  const now = useNow(running ? 1000 : 60_000);
  return (
    <div>
      <CronRunsListHeader
        count={
          <span
            className={CRON_RUNS_COUNT_CLASS}
            aria-label={t("services.cronRunsCount", { count: runs.length })}
          >
            {runs.length.toLocaleString(i18n.language)}
          </span>
        }
      />
      <ul className="divide-y">
        {runs.map((run) => (
          <CronRunRow
            key={run.id}
            serviceId={serviceId}
            run={run}
            latestSuccessful={run.id === latestSuccessfulId}
            viewerEmail={viewerEmail}
            now={now}
            cancel={
              cronRunStatus(run) === "pending" ? (
                <PermissionTooltip reason={cancelBlocked}>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive hover:text-destructive max-lg:h-11"
                    disabled={cancelingId === run.id || !!cancelBlocked}
                    onClick={() => onCancel(run)}
                  >
                    {cancelingId === run.id ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <X />
                    )}
                    {t("services.cronRunCancel")}
                  </Button>
                </PermissionTooltip>
              ) : null
            }
          />
        ))}
      </ul>
    </div>
  );
}

// Kratos identity traits: bex's schema is `{ email, name? }`.
function useViewerEmail(): string | null {
  const { session } = useRootContext();
  const traits = session?.identity?.traits as { email?: string } | undefined;
  return traits?.email ?? null;
}

function CronRunRow({
  serviceId,
  run,
  latestSuccessful,
  viewerEmail,
  now,
  cancel,
}: {
  serviceId: string;
  run: CronRunView;
  latestSuccessful: boolean;
  viewerEmail: string | null;
  now: number;
  cancel: React.ReactNode;
}) {
  const { t, i18n } = useTranslations();
  const status = cronRunStatus(run);
  const label = statusLabel(status, latestSuccessful, t);
  const logs = cronRunLogsSearch(run, now);
  const elapsed = cronRunElapsedMs(run, now);
  const duration =
    elapsed === null ? "—" : formatDuration(elapsed, i18n.language);
  const trigger = triggerLabel(run, t);
  const ago = run.startedAt
    ? formatTimeAgo(run.startedAt, {
        now,
        language: i18n.language,
        justNow: t("common.justNow"),
      })
    : null;
  const ranAt = ago
    ? t("services.cronRunStartedAgo", { timestamp: ago })
    : null;
  const isViewer = (email: string | null) =>
    Boolean(email && viewerEmail && email === viewerEmail);
  const canceledBy =
    status !== "canceled"
      ? null
      : run.canceledBy
        ? isViewer(run.canceledBy)
          ? t("services.cronRunCanceledByYou")
          : t("services.cronRunCanceledBy", { user: run.canceledBy })
        : t("services.cronRunStatusCanceled");
  const logsLink = {
    to: "/services/$serviceId/logs",
    params: { serviceId },
    search: logs,
  } as const;

  return (
    <li
      className={cn(
        cronRunRowGrid,
        CRON_RUN_ROW_CLASS,
        "group/row hover:bg-muted/40 active:bg-muted/60",
      )}
    >
      <div className={CRON_RUN_ICON_CELL}>
        {/* Render's icon opens the same Logs, with the status in a tooltip.
            The row's link names the status for keyboard and assistive tech,
            so the icon stays out of the Tab order. */}
        <Tooltip>
          <TooltipTrigger asChild>
            <Link
              {...logsLink}
              tabIndex={-1}
              aria-hidden="true"
              className="relative z-10 rounded-full"
              suppressHydrationWarning
            >
              <StatusKindIcon
                kind={cronRunIconKind(status, latestSuccessful)}
              />
            </Link>
          </TooltipTrigger>
          <TooltipContent>{label}</TooltipContent>
        </Tooltip>
      </div>
      <div className={CRON_RUN_MAIN_CELL}>
        {/* The whole row opens the Logs over the run's window; its ::after
            covers the row, and the time and Cancel sit above it. */}
        <Link
          {...logsLink}
          className={cn("min-w-0 after:rounded-md", STRETCHED_LINK_CLASS)}
          suppressHydrationWarning
        >
          <span className="block h-5 truncate text-sm font-medium text-foreground underline-offset-2 group-hover/row:underline">
            {t("services.cronRunTitle")}
          </span>
          <span className="sr-only" suppressHydrationWarning>
            {[label, ranAt].filter(Boolean).join(", ")}
          </span>
        </Link>
        {ranAt || canceledBy ? (
          <p
            className={cn(
              CRON_RUN_LINE,
              "flex-wrap gap-x-1 text-muted-foreground",
            )}
          >
            {ranAt && run.startedAt ? (
              <InstantTooltip
                value={run.startedAt}
                // A touch has no hover: a tap falls through to the row.
                className="relative z-10 no-underline hover:underline pointer-coarse:pointer-events-none"
              >
                {ranAt}
              </InstantTooltip>
            ) : null}
            {ranAt && run.triggeredBy ? (
              <span className="min-w-0 break-all">
                {isViewer(run.triggeredBy)
                  ? t("services.cronRunByYou")
                  : t("services.cronRunBy", { user: run.triggeredBy })}
              </span>
            ) : null}
            {canceledBy ? (
              <span className="min-w-0 break-all">
                {ranAt ? (
                  <span aria-hidden="true" className="mr-1">
                    ·
                  </span>
                ) : null}
                {canceledBy}
              </span>
            ) : null}
          </p>
        ) : null}
        {/* Below the wide list, Trigger and Duration fold into a line. */}
        <p
          className={cn(
            CRON_RUN_LINE,
            "whitespace-nowrap text-muted-foreground @3xl/runs:hidden",
          )}
        >
          <span className="truncate">{trigger}</span>
          <span
            className="shrink-0 tabular-nums before:mx-1.5 before:content-['·']"
            suppressHydrationWarning
          >
            {duration}
          </span>
        </p>
      </div>
      <p className="hidden truncate text-sm text-muted-foreground @3xl/runs:block">
        <span className="sr-only">{t("services.cronRunsColTrigger")}: </span>
        {trigger}
      </p>
      <p
        className="hidden text-sm whitespace-nowrap tabular-nums text-muted-foreground @3xl/runs:block"
        suppressHydrationWarning
      >
        <span className="sr-only">{t("services.cronRunColDuration")}: </span>
        {duration}
      </p>
      <div className="relative z-10 flex justify-end empty:hidden @3xl/runs:empty:block">
        {cancel}
      </div>
    </li>
  );
}
