import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  ServiceEventsDocument,
  TriggerDeployDocument,
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

type ConfirmAction =
  | { kind: "manual" }
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

  const [triggerDeploy, { loading: triggering }] =
    useMutation(TriggerDeployDocument);
  const [cancelDeploy, { loading: canceling }] =
    useMutation(CancelDeployDocument);
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
  );

  const busy = triggering || canceling || rollingBack;

  const events = (data?.serviceEvents ?? []).filter(
    (e): e is NonNullable<typeof e> & { id: string } => e != null && !!e.id,
  );

  async function handleConfirm() {
    if (!confirm) return;
    try {
      if (confirm.kind === "manual") {
        await triggerDeploy({ variables: { serviceId } });
        toast.success(t("services.triggerDeploySuccess"));
      } else if (confirm.kind === "cancel") {
        await cancelDeploy({
          variables: { serviceId, deployId: confirm.deployId },
        });
        toast.success(t("services.cancelDeploySuccess"));
      } else if (confirm.kind === "rollback") {
        await rollbackService({
          variables: { serviceId, deployId: confirm.deployId },
        });
        toast.success(t("services.rollbackSuccess"));
      }
      void refetch();
    } catch {
      if (confirm.kind === "manual")
        toast.error(t("services.triggerDeployError"));
      else if (confirm.kind === "cancel")
        toast.error(t("services.cancelDeployError"));
      else toast.error(t("services.rollbackError"));
    } finally {
      setConfirm(null);
    }
  }

  const confirmTitle =
    confirm?.kind === "manual"
      ? t("services.eventsManualDeployConfirmTitle")
      : confirm?.kind === "cancel"
        ? t("services.eventsCancelConfirmTitle")
        : t("services.eventsRollbackConfirmTitle");

  const confirmBody =
    confirm?.kind === "manual"
      ? t("services.eventsManualDeployConfirmBody")
      : confirm?.kind === "cancel"
        ? t("services.eventsCancelConfirmBody")
        : t("services.eventsRollbackConfirmBody");

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>{t("services.eventsTitle")}</CardTitle>
          <Button
            size="sm"
            disabled={busy}
            onClick={() => setConfirm({ kind: "manual" })}
          >
            {t("services.eventsManualDeploy")}
          </Button>
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
