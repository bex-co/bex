import { useState } from "react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
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

/** Cursor-paged cron run objects with a confirmed cancel action for pending rows. */
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
  } = useCronRuns(serviceId);
  const [confirmRun, setConfirmRun] = useState<CronRunView | null>(null);

  async function handleCancel() {
    if (!confirmRun) return;
    await cancel(confirmRun.id);
    setConfirmRun(null);
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("services.cronRunsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
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
                  {runs.map((run) => (
                    <TableRow key={run.id}>
                      <TableCell className="tabular-nums text-muted-foreground">
                        {formatRelativeAge(run.startedAt)}
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
                  ))}
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
    </>
  );
}
