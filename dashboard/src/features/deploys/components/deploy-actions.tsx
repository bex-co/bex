import { useState } from "react";
import { useMutation } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  CancelDeployDocument,
  RollbackServiceDocument,
} from "@/graphql/definitions";
import { Button } from "@/common/components/ui/button";
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
import { useTranslations } from "@/common/hooks/use-translations";
import {
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
} from "@/features/deploys/lib/deploy-status";

type ConfirmAction = "cancel" | "rollback";

export interface DeployActionsProps {
  serviceId: string;
  deployId: string;
  status: string;
  onChanged?: () => void;
}

/**
 * The shared Cancel/Rollback controls used by both the Events list and deploy
 * detail page. Mutation, confirmation, toast, and post-rollback navigation
 * live here so the two surfaces cannot drift in behavior.
 */
export function DeployActions({
  serviceId,
  deployId,
  status,
  onChanged,
}: DeployActionsProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  // Both verbs change the deploy history and the events feed: rollback mints a
  // brand-new deploy (which no cached list contains) and cancel appends an
  // event row. Refetch the active `Deploys` (history tab + the service
  // header's latest-deploy chrome) and `ServiceEvents` queries by name so
  // every mounted surface converges without each caller having to know.
  const [cancelDeploy, { loading: canceling }] = useMutation(
    CancelDeployDocument,
    { refetchQueries: ["Deploys", "ServiceEvents"] },
  );
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
    { refetchQueries: ["Deploys", "ServiceEvents"] },
  );
  const busy = canceling || rollingBack;

  async function handleConfirm() {
    if (!confirm) return;
    try {
      if (confirm === "cancel") {
        await cancelDeploy({ variables: { serviceId, deployId } });
        toast.success(t("services.cancelDeploySuccess"));
      } else {
        const { data } = await rollbackService({
          variables: { serviceId, deployId },
        });
        const rollbackId = data?.rollbackService?.id;
        if (!rollbackId)
          throw new Error("rollbackService returned no deploy id");
        toast.success(t("services.rollbackSuccess"));
        void navigate({
          to: "/services/$serviceId/deploys/$deployId",
          params: { serviceId, deployId: rollbackId },
        });
      }
      onChanged?.();
    } catch {
      toast.error(
        confirm === "cancel"
          ? t("services.cancelDeployError")
          : t("services.rollbackError"),
      );
    } finally {
      setConfirm(null);
    }
  }

  const canCancel = isCancelableDeployStatus(status);
  const canRollback = isRollbackableDeployStatus(status);
  if (!canCancel && !canRollback) return null;

  return (
    <>
      <div className="flex shrink-0 gap-2">
        {canCancel ? (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setConfirm("cancel")}
          >
            {t("services.eventsCancelDeploy")}
          </Button>
        ) : null}
        {canRollback ? (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setConfirm("rollback")}
          >
            {t("services.eventsRollback")}
          </Button>
        ) : null}
      </div>

      <AlertDialog
        open={confirm !== null}
        onOpenChange={(open) => !open && setConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm === "cancel"
                ? t("services.eventsCancelConfirmTitle")
                : t("services.eventsRollbackConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm === "cancel"
                ? t("services.eventsCancelConfirmBody")
                : t("services.eventsRollbackConfirmBody")}
            </AlertDialogDescription>
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
    </>
  );
}
