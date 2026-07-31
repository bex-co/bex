import { Fragment, useState } from "react";
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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
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
import { formatRelativeAge } from "@/features/services/lib/format";
import type {
  CronRunView,
  ServiceBadgeVariant,
} from "@/features/services/types";
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
            <Button
              size="sm"
              disabled={triggering || hasActiveRun}
              title={hasActiveRun ? t("services.cronTriggerActive") : undefined}
              onClick={() => {
                clearTriggerError();
                setConfirmTrigger(true);
              }}
            >
              {triggering
                ? t("services.cronTriggering")
                : t("services.cronTriggerRun")}
            </Button>
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
                              aria-label={t("services.cronRunDetailToggle")}
                              onClick={() =>
                                setExpandedRunId(expanded ? null : run.id)
                              }
                            >
                              {expanded ? (
                                <ChevronDown className="h-3.5 w-3.5" />
                              ) : (
                                <ChevronRight className="h-3.5 w-3.5" />
                              )}
                              {formatRelativeAge(run.startedAt)}
                            </button>
                          </TableCell>
                          <TableCell className="tabular-nums text-muted-foreground">
                            {duration(run)}
                          </TableCell>
                          <TableCell>
                            <CronRunStatusBadge status={run.status} />
                          </TableCell>
                          <TableCell className="text-right">
                            {run.status.toLowerCase() === "pending" ||
                            run.status.toLowerCase() === "running" ? (
                              <Button
                                size="sm"
                                variant="outline"
                                disabled={cancelingId === run.id}
                                onClick={() => setConfirmRun(run)}
                              >
                                {t("services.cronRunCancel")}
                              </Button>
                            ) : null}
                          </TableCell>
                        </TableRow>
                        {expanded ? (
                          <TableRow>
                            <TableCell colSpan={4} className="bg-muted/30">
                              <CronRunDetail
                                serviceId={serviceId}
                                runId={run.id}
                              />
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

      <AlertDialog
        open={confirmRun !== null}
        onOpenChange={(open) => !open && setConfirmRun(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("services.cronRunCancelConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.cronRunCancelConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.eventsConfirmCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={confirmRun ? cancelingId === confirmRun.id : false}
              onClick={() => void handleCancel()}
            >
              {t("services.eventsConfirmProceed")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={confirmTrigger}
        onOpenChange={(open) => !open && setConfirmTrigger(false)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("services.cronTriggerConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.cronTriggerConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.eventsConfirmCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={triggering}
              onClick={() => void handleTrigger()}
            >
              {t("services.cronTriggerRun")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
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
 */
function CronRunDetail({
  serviceId,
  runId,
}: {
  serviceId: string;
  runId: string;
}) {
  const { t } = useTranslations();
  const { run, loading, error } = useCronRun(serviceId, runId);

  if (loading && !run) {
    return <Skeleton className="h-16 w-full" />;
  }
  if (error || !run) {
    return (
      <p className="text-sm text-destructive">
        {t("services.cronRunDetailError")}
      </p>
    );
  }
  return (
    <dl className="grid grid-cols-2 gap-x-6 gap-y-1 text-sm sm:grid-cols-4">
      <DetailField label={t("services.cronRunColStatus")}>
        <CronRunStatusBadge status={run.status} />
      </DetailField>
      <DetailField label={t("services.cronRunDetailStarted")}>
        <span className="tabular-nums">{absoluteTime(run.startedAt)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunDetailFinished")}>
        <span className="tabular-nums">{absoluteTime(run.finishedAt)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunColDuration")}>
        <span className="tabular-nums">{duration(run)}</span>
      </DetailField>
      <DetailField label={t("services.cronRunDetailId")}>
        <span className="font-mono text-xs">{run.id}</span>
      </DetailField>
    </dl>
  );
}

function DetailField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}
