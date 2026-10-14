import { Fragment, useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCronRuns } from "@/features/services/hooks/use-cron-runs";
import { useCronRun } from "@/features/services/hooks/use-cron-run";
import { RelativeAge } from "@/common/components/relative-time";
import type {
  CronRunView,
  ServiceBadgeVariant,
} from "@/features/services/types";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import {
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { en } from "@/i18n";

const RUN_STATUS: Record<
  string,
  { label: keyof typeof en; variant: ServiceBadgeVariant }
> = {
  pending: { label: "services.cronRunStatusRunning", variant: "outline" },
  running: { label: "services.cronRunStatusRunning", variant: "outline" },
  successful: {
    label: "services.cronRunStatusSucceeded",
    variant: "default",
  },
  succeeded: {
    label: "services.cronRunStatusSucceeded",
    variant: "default",
  },
  unsuccessful: {
    label: "services.cronRunStatusFailed",
    variant: "destructive",
  },
  failed: {
    label: "services.cronRunStatusFailed",
    variant: "destructive",
  },
  canceled: {
    label: "services.cronRunStatusCanceled",
    variant: "secondary",
  },
};

// The same active set useCronRuns' hasActiveRun uses.
const ACTIVE_STATUSES = new Set(["pending", "running"]);

function CronRunStatusBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  const value = RUN_STATUS[status.toLowerCase()] ?? RUN_STATUS.pending;
  return <Badge variant={value.variant}>{t(value.label)}</Badge>;
}

function duration(run: CronRunView): string {
  if (!run.startedAt || !run.finishedAt) return "—";
  const milliseconds =
    new Date(run.finishedAt).getTime() - new Date(run.startedAt).getTime();
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return "—";
  const seconds = Math.round(milliseconds / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
}

/** Cursor-paged cron run objects with Trigger Run, per-run detail, and a
 *  confirmed cancel action for pending rows. */
export function CronRunsSection({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const {
    runs,
    loading,
    error,
    loadingMore,
    hasMore,
    cancelingId,
    loadMore,
    cancel,
    hasActiveRun,
    triggering,
    triggerError,
    clearTriggerError,
    trigger,
  } = useCronRuns(serviceId);
  // The server's own decision gates both verbs (w4/203): a suspended cron
  // answers cron_run_now with precondition "suspended", which the button
  // used to ignore and then show the raw refusal.
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(serviceId);
  // The action snapshot is a separate no-cache read that history polling never
  // replaces, so a run arriving after it kept a settled "no active run" Cancel
  // (w4/249). Re-read it whenever the set of active runs changes, and treat
  // Cancel as checking until that re-read settles.
  const activeRunKey = runs
    .filter((run) => ACTIVE_STATUSES.has(run.status.toLowerCase()))
    .map((run) => run.id)
    .join(" ");
  const [checkedRunKey, setCheckedRunKey] = useState(activeRunKey);
  const refreshActions = useRef(serverActions.refresh);
  refreshActions.current = serverActions.refresh;
  useEffect(() => {
    if (activeRunKey === checkedRunKey) return;
    let live = true;
    void refreshActions.current().finally(() => {
      if (live) setCheckedRunKey(activeRunKey);
    });
    return () => {
      live = false;
    };
  }, [activeRunKey, checkedRunKey]);
  const actionBlocked = (
    action: ResourceActionId,
    status = serverActions.status,
  ) =>
    gateReason(
      gateAction(
        serverActions.status === "ready"
          ? resourceDecision(
              serverActions.snapshot,
              currentWorkspaceId,
              serviceId,
              action,
            )
          : null,
        status,
      ),
      t,
    );
  const triggerBlocked = actionBlocked("cron_run_now");
  const cancelBlocked = actionBlocked(
    "cron_cancel_run",
    activeRunKey === checkedRunKey ? serverActions.status : "checking",
  );
  const [confirmRun, setConfirmRun] = useState<CronRunView | null>(null);
  const [confirmTrigger, setConfirmTrigger] = useState(false);
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);

  async function handleCancel() {
    if (!confirmRun) return;
    await cancel(confirmRun.id);
    setConfirmRun(null);
  }

  async function handleTrigger() {
    const ok = await trigger();
    setConfirmTrigger(false);
    if (ok) setExpandedRunId(null);
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("services.cronRunsTitle")}</CardTitle>
          <CardAction>
            {/* w4/m114 t003: Trigger Run stays enabled during an active run.
                The server preempts by design — runCronJob cancels the active
                Job and returns a new pending run — so disabling the button
                here forbade what the API performs, and blocked the one
                in-product recovery for a wedged run (the QA probe had to drop
                to raw GraphQL). The confirm dialog below names the
                preemption instead. */}
            <PermissionTooltip reason={triggerBlocked}>
              <Button
                size="sm"
                disabled={triggering || !!triggerBlocked}
                onClick={() => {
                  clearTriggerError();
                  setConfirmTrigger(true);
                }}
              >
                {triggering
                  ? t("services.cronTriggering")
                  : t("services.cronTriggerRun")}
              </Button>
            </PermissionTooltip>
          </CardAction>
        </CardHeader>
        <CardContent>
          {triggerError ? (
            <p className="mb-4 text-sm text-destructive">{triggerError}</p>
          ) : null}
          {loading && runs.length === 0 ? (
            <div className="space-y-2">
              {[0, 1, 2].map((row) => (
                <Skeleton key={row} className="h-10 w-full" />
              ))}
            </div>
          ) : error && runs.length === 0 ? (
            <p className="text-sm text-destructive">
              {t("services.cronRunsLoadError")}
            </p>
          ) : runs.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("services.cronRunsEmpty")}
            </p>
          ) : (
            <div className="space-y-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("services.cronRunColStarted")}</TableHead>
                    <TableHead>{t("services.cronRunColDuration")}</TableHead>
                    <TableHead>{t("services.cronRunColStatus")}</TableHead>
                    <TableHead className="text-right">
                      {t("services.cronRunColActions")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {runs.map((run) => {
                    const expanded = expandedRunId === run.id;
                    return (
                      <Fragment key={run.id}>
                        <TableRow>
                          <TableCell className="tabular-nums text-muted-foreground">
                            <button
                              type="button"
                              className="flex items-center gap-1 hover:text-foreground"
                              aria-expanded={expanded}
                              onClick={() =>
                                setExpandedRunId(expanded ? null : run.id)
                              }
                            >
                              {expanded ? (
                                <ChevronDown className="h-3.5 w-3.5" />
                              ) : (
                                <ChevronRight className="h-3.5 w-3.5" />
                              )}
                              <RelativeAge value={run.startedAt} />
                              {/* The name follows the visible age, so each row
                                  names its own run (w4/213: every button was
                                  "Toggle run detail"). */}
                              <span className="sr-only">
                                {t("services.cronRunDetailToggle", {
                                  when: runToggleWhen(run, runs),
                                })}
                              </span>
                            </button>
                          </TableCell>
                          <TableCell className="tabular-nums text-muted-foreground">
                            {duration(run)}
                          </TableCell>
                          <TableCell>
                            <CronRunStatusBadge status={run.status} />
                          </TableCell>
                          <TableCell className="text-right">
                            {ACTIVE_STATUSES.has(run.status.toLowerCase()) ? (
                              <PermissionTooltip reason={cancelBlocked}>
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={
                                    cancelingId === run.id || !!cancelBlocked
                                  }
                                  onClick={() => setConfirmRun(run)}
                                >
                                  {t("services.cronRunCancel")}
                                </Button>
                              </PermissionTooltip>
                            ) : null}
                          </TableCell>
                        </TableRow>
                        {expanded ? (
                          <TableRow>
                            <TableCell colSpan={4} className="bg-muted/30">
                              <CronRunDetail serviceId={serviceId} run={run} />
                            </TableCell>
                          </TableRow>
                        ) : null}
                      </Fragment>
                    );
                  })}
                </TableBody>
              </Table>
              {hasMore ? (
                <Button
                  variant="outline"
                  disabled={loadingMore}
                  onClick={() => void loadMore()}
                >
                  {loadingMore
                    ? t("services.cronRunsLoadingMore")
                    : t("services.cronRunsLoadMore")}
                </Button>
              ) : null}
            </div>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirmRun !== null}
        onOpenChange={(open) => !open && setConfirmRun(null)}
        title={t("services.cronRunCancelConfirmTitle")}
        description={t("services.cronRunCancelConfirmBody")}
        cancelLabel={t("services.eventsConfirmCancel")}
        confirmLabel={t("services.eventsConfirmProceed")}
        pending={confirmRun ? cancelingId === confirmRun.id : false}
        onConfirm={() => void handleCancel()}
      />

      <ConfirmDialog
        open={confirmTrigger}
        onOpenChange={(open) => !open && setConfirmTrigger(false)}
        title={t("services.cronTriggerConfirmTitle")}
        description={
          hasActiveRun
            ? t("services.cronTriggerConfirmBodyPreempt")
            : t("services.cronTriggerConfirmBody")
        }
        cancelLabel={t("services.eventsConfirmCancel")}
        confirmLabel={t("services.cronTriggerRun")}
        // Triggering a run is not destructive — it is the primary action here.
        destructive={false}
        pending={triggering}
        onConfirm={() => void handleTrigger()}
      />
    </>
  );
}

/** What a run's detail toggle names it by: its absolute start time, with the
 *  run id when the time is missing or another listed run shares it. */
function runToggleWhen(run: CronRunView, runs: CronRunView[]): string {
  const when = absoluteTime(run.startedAt);
  if (when === "—") return run.id;
  const shared = runs.some(
    (other) => other.id !== run.id && absoluteTime(other.startedAt) === when,
  );
  return shared ? `${when} (${run.id})` : when;
}

/** Absolute local timestamp, or an em dash when the run hasn't reached it yet. */
function absoluteTime(value: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

/**
 * One run's detail, freshly read via `cronJobRun` (w5/m60) when its row is
 * expanded: status, absolute start/finish timestamps, computed duration, and the
 * run id. A stale/unknown run id renders an explicit error, never a blank panel.
 *
 * While that first read is in flight the panel renders the same five-field grid
 * with every value masked (w4/305): the history row's run sizes the hidden
 * values so columns, wrapping and table widths match the ready detail, but none
 * of it is shown or announced as the fresh detail.
 */
function CronRunDetail({
  serviceId,
  run: historyRun,
}: {
  serviceId: string;
  run: CronRunView;
}) {
  const { t } = useTranslations();
  const { run, loading, error } = useCronRun(serviceId, historyRun.id);

  if (loading && !run) {
    return (
      <>
        <span role="status" className="sr-only">
          {t("services.cronRunDetailLoading")}
        </span>
        <CronRunFields run={historyRun} masked />
      </>
    );
  }
  if (error || !run) {
    return (
      <p className="text-sm text-destructive">
        {t("services.cronRunDetailError")}
      </p>
    );
  }
  return <CronRunFields run={run} masked={false} />;
}

/** The detail's five-field grid — one layout for the masked preview and the
 *  ready detail, so the two cannot drift apart. */
function CronRunFields({ run, masked }: { run: CronRunView; masked: boolean }) {
  const { t } = useTranslations();
  return (
    <dl
      aria-busy={masked || undefined}
      className="grid grid-cols-2 gap-x-6 gap-y-1 text-sm sm:grid-cols-4"
    >
      <DetailField label={t("services.cronRunColStatus")} masked={masked}>
        <CronRunStatusBadge status={run.status} />
      </DetailField>
      <DetailField label={t("services.cronRunDetailStarted")} masked={masked}>
        <span className="tabular-nums">{absoluteTime(run.startedAt)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunDetailFinished")} masked={masked}>
        <span className="tabular-nums">{absoluteTime(run.finishedAt)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunColDuration")} masked={masked}>
        <span className="tabular-nums">{duration(run)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunDetailId")} masked={masked}>
        <span className="font-mono text-xs">{run.id}</span>
      </DetailField>
    </dl>
  );
}

/** A labelled value. Masked, the value still lays out — sizing the cell exactly
 *  as the ready value would — but is invisible, hidden from assistive tech, and
 *  covered by a skeleton. */
function DetailField({
  label,
  masked,
  children,
}: {
  label: string;
  masked: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      {masked ? (
        <dd className="relative">
          <span aria-hidden="true" className="invisible">
            {children}
          </span>
          <Skeleton aria-hidden="true" className="absolute inset-0" />
        </dd>
      ) : (
        <dd>{children}</dd>
      )}
    </div>
  );
}
