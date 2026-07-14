import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  ServiceEventsDocument,
  CancelDeployDocument,
  RollbackServiceDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
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
import { useServer } from "@/features/services/hooks/use-server";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import { isCron } from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/events")({
  component: ServiceEventsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Events · bex dashboard` }],
  }),
});

// Map wire deployStatus to badge variant + locale key
function statusVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "live":
      return "default";
    case "update_in_progress":
      return "secondary";
    case "update_failed":
      return "destructive";
    case "canceled":
      return "outline";
    default:
      return "secondary";
  }
}

function statusKey(status: string): string {
  switch (status) {
    case "live":
      return "services.eventsStatusLive";
    case "update_in_progress":
      return "services.eventsStatusInProgress";
    case "update_failed":
      return "services.eventsStatusFailed";
    case "canceled":
      return "services.eventsStatusCanceled";
    default:
      return status;
  }
}

// The pre-deploy step's own status line (w1/m33), shown under the deploy badge
// so a migration failure reads distinctly from a health-check failure. Only
// running/succeeded/failed carry a label; "" (no pre-deploy step) shows nothing.
function preDeployKey(status: string): string | null {
  switch (status) {
    case "running":
      return "services.eventsPreDeployRunning";
    case "succeeded":
      return "services.eventsPreDeploySucceeded";
    case "failed":
      return "services.eventsPreDeployFailed";
    default:
      return null;
  }
}

// ServiceEventDetails.trigger is the Trigger object (boolean flags), a
// different shape from Deploy.trigger's plain string — derive a short label
// from whichever flag is set, first-match-wins.
type TriggerFlags = {
  firstBuild?: boolean | null;
  envUpdated?: boolean | null;
  manual?: boolean | null;
  deployedByRender?: boolean | null;
  clearCache?: boolean | null;
  rollback?: boolean | null;
} | null;

function triggerLabel(trigger: TriggerFlags): string | null {
  if (!trigger) return null;
  if (trigger.rollback) return "rollback";
  if (trigger.firstBuild) return "first build";
  if (trigger.manual) return "manual";
  if (trigger.envUpdated) return "env updated";
  if (trigger.clearCache) return "clear cache";
  if (trigger.deployedByRender) return "deployed by render";
  return null;
}

// Manual Deploy is not here: it's a header verb on Render, so it lives in
// ServiceDetailHeader (via ManualDeployButton) and refetches this page's
// `ServiceEvents` query by name when it fires.
type ConfirmAction =
  | { kind: "cancel"; deployId: string }
  | { kind: "rollback"; deployId: string };

export function ServiceEventsPage() {
  const { serviceId } = Route.useParams();
  const { t } = useTranslations();
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);

  const { data, loading, refetch } = useQuery(ServiceEventsDocument, {
    variables: { serviceId, limit: 20 },
    fetchPolicy: "cache-and-network",
  });

  // A cron_job's run history hangs off the same landing tab (it was the retired
  // Overview panel's second card); `server(id)` is already in Apollo's cache
  // from the detail shell's own read, so this doesn't cost a second request.
  const { service } = useServer(serviceId);

  const [cancelDeploy, { loading: canceling }] =
    useMutation(CancelDeployDocument);
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
  );

  const busy = canceling || rollingBack;

  const events = (data?.serviceEvents ?? []).filter(
    (e): e is NonNullable<typeof e> & { id: string } => e != null && !!e.id,
  );

  async function handleConfirm() {
    if (!confirm) return;
    try {
      if (confirm.kind === "cancel") {
        await cancelDeploy({
          variables: { serviceId, deployId: confirm.deployId },
        });
        toast.success(t("services.cancelDeploySuccess"));
      } else {
        await rollbackService({
          variables: { serviceId, deployId: confirm.deployId },
        });
        toast.success(t("services.rollbackSuccess"));
      }
      void refetch();
    } catch {
      if (confirm.kind === "cancel")
        toast.error(t("services.cancelDeployError"));
      else toast.error(t("services.rollbackError"));
    } finally {
      setConfirm(null);
    }
  }

  const confirmTitle =
    confirm?.kind === "cancel"
      ? t("services.eventsCancelConfirmTitle")
      : t("services.eventsRollbackConfirmTitle");

  const confirmBody =
    confirm?.kind === "cancel"
      ? t("services.eventsCancelConfirmBody")
      : t("services.eventsRollbackConfirmBody");

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("services.eventsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading && events.length === 0 ? (
            <div className="space-y-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("services.eventsEmpty")}
            </p>
          ) : (
            <div className="divide-y">
              {events.map((evt) => {
                const details = evt.details;
                const isInProgress =
                  details?.deployStatus === "update_in_progress";
                const canRollback = details?.deployStatus === "live";
                const label = triggerLabel(details?.trigger ?? null);
                const preDeploy = preDeployKey(details?.preDeployStatus ?? "");
                return (
                  <div
                    key={evt.id}
                    className="flex items-start justify-between gap-4 py-3"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <Badge
                          variant={statusVariant(details?.deployStatus ?? "")}
                        >
                          {t(
                            statusKey(
                              details?.deployStatus ?? "",
                            ) as Parameters<typeof t>[0],
                          )}
                        </Badge>
                        {label && (
                          <span className="text-xs capitalize text-muted-foreground">
                            {label}
                          </span>
                        )}
                      </div>
                      {preDeploy && (
                        <p
                          className={`mt-1 text-xs ${
                            details?.preDeployStatus === "failed"
                              ? "text-destructive"
                              : "text-muted-foreground"
                          }`}
                        >
                          {t(preDeploy as Parameters<typeof t>[0])}
                        </p>
                      )}
                      {evt.timestamp && (
                        <p className="mt-1 text-xs text-muted-foreground">
                          {new Date(evt.timestamp).toLocaleString()}
                        </p>
                      )}
                    </div>
                    <div className="flex shrink-0 gap-2">
                      {isInProgress && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy}
                          onClick={() =>
                            setConfirm({ kind: "cancel", deployId: evt.id })
                          }
                        >
                          {t("services.eventsCancelDeploy")}
                        </Button>
                      )}
                      {canRollback && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy}
                          onClick={() =>
                            setConfirm({ kind: "rollback", deployId: evt.id })
                          }
                        >
                          {t("services.eventsRollback")}
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {service && isCron(service) ? (
        <CronRunsSection service={service} />
      ) : null}

      <AlertDialog
        open={confirm !== null}
        onOpenChange={(o) => !o && setConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>{confirmBody}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.eventsConfirmCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => void handleConfirm()}
              disabled={busy}
            >
              {t("services.eventsConfirmProceed")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
