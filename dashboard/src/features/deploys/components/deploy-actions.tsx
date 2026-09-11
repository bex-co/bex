import { useMutation } from "@apollo/client/react";
import { useNavigate } from "@tanstack/react-router";
import { RotateCcw } from "lucide-react";
import { toast } from "sonner";
import {
  CancelDeployDocument,
  RollbackServiceDocument,
} from "@/graphql/definitions";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { DEPLOY_REFETCH_QUERIES } from "@/common/lib/fetch-policy";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceBase } from "@/features/services/lib/service-base";
import {
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  decisionForSelectedRollback,
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";

type ConfirmAction = "cancel" | "rollback";

export interface DeployActionsProps {
  serviceId: string;
  deployId: string;
  status: string;
  onChanged?: () => void;
}

/**
 * Shared Cancel/Rollback controls for Events list and deploy detail.
 * Status eligibility binds the selected deploy; operation permission and
 * shared preconditions come from deployActions. Service-wide
 * no_eligible_rollback_target does not disable an older selected eligible
 * row (w6/m143/t003).
 */
export function DeployActions({
  serviceId,
  deployId,
  status,
  onChanged,
}: DeployActionsProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const base = useServiceBase();
  const { currentWorkspaceId } = useWorkspace();
  const deployActions = useDeployActions(serviceId);
  const { pending, openConfirm, clearConfirm, recheckBeforeDispatch } =
    useBoundActionConfirm({ resourceId: serviceId, deployId });
  const [cancelDeploy, { loading: canceling }] = useMutation(
    CancelDeployDocument,
    { refetchQueries: DEPLOY_REFETCH_QUERIES },
  );
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
    { refetchQueries: DEPLOY_REFETCH_QUERIES },
  );
  const busy = canceling || rollingBack;

  const statusCancel = isCancelableDeployStatus(status);
  const statusRollback = isRollbackableDeployStatus(status);

  function reasonFor(action: "cancel_deploy" | "rollback"): string | undefined {
    if (deployActions.status !== "ready") {
      return gateReason(gateAction(null, deployActions.status), t);
    }
    let decision = resourceDecision(
      deployActions.snapshot,
      currentWorkspaceId,
      serviceId,
      action,
    );
    // Exact-target eligibility: do not let the latest-20 summary disable a
    // selected row that itself is rollbackable by status.
    if (action === "rollback") {
      decision = decisionForSelectedRollback(decision);
    }
    // Cancel: selected-row status is authoritative for "is this deploy open";
    // ignore a stale service-wide no_active_deploy when this row is cancelable.
    if (
      action === "cancel_deploy" &&
      decision?.outcome === "allowed" &&
      decision.precondition === "no_active_deploy" &&
      statusCancel
    ) {
      decision = { ...decision, precondition: "" };
    }
    return gateReason(gateAction(decision, "ready"), t);
  }

  const cancelReason = statusCancel ? reasonFor("cancel_deploy") : undefined;
  const rollbackReason = statusRollback ? reasonFor("rollback") : undefined;

  const confirm: ConfirmAction | null =
    pending?.action === "cancel_deploy"
      ? "cancel"
      : pending?.action === "rollback"
        ? "rollback"
        : null;

  async function handleConfirm() {
    const { ok, binding } = await recheckBeforeDispatch();
    if (!ok || !binding) return;
    const action = binding.action;
    try {
      if (action === "cancel_deploy") {
        await cancelDeploy({
          variables: { serviceId, deployId: binding.deployId ?? deployId },
        });
        toast.success(t("services.cancelDeploySuccess"));
      } else if (action === "rollback") {
        const { data } = await rollbackService({
          variables: {
            serviceId,
            deployId: binding.deployId ?? deployId,
          },
        });
        const rollbackId = data?.rollbackService?.id;
        if (!rollbackId)
          throw new Error("rollbackService returned no deploy id");
        toast.success(t("services.rollbackSuccess"));
        void navigate({
          to: `${base}/$serviceId/deploys/$deployId`,
          params: { serviceId, deployId: rollbackId },
        });
      }
      onChanged?.();
    } catch {
      toast.error(
        action === "cancel_deploy"
          ? t("services.cancelDeployError")
          : t("services.rollbackError"),
      );
    } finally {
      clearConfirm();
    }
  }

  function open(action: ConfirmAction) {
    const id: ResourceActionId =
      action === "cancel" ? "cancel_deploy" : "rollback";
    const reason = action === "cancel" ? cancelReason : rollbackReason;
    if (reason) return;
    openConfirm(id);
  }

  if (!statusCancel && !statusRollback) return null;

  return (
    <>
      <div className="flex shrink-0 gap-2">
        {statusCancel ? (
          <PermissionTooltip reason={cancelReason}>
            <Button
              size="sm"
              variant="outline"
              disabled={busy || !!cancelReason}
              onClick={() => open("cancel")}
            >
              {t("services.eventsCancelDeploy")}
            </Button>
          </PermissionTooltip>
        ) : null}
        {statusRollback ? (
          <PermissionTooltip reason={rollbackReason}>
            <Button
              size="sm"
              variant="link"
              disabled={busy || !!rollbackReason}
              onClick={() => open("rollback")}
              className="h-8 gap-1.5 px-0 text-muted-foreground hover:text-foreground"
            >
              <RotateCcw />
              {t("services.eventsRollback")}
            </Button>
          </PermissionTooltip>
        ) : null}
      </div>

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(openState) => !openState && clearConfirm()}
        title={
          confirm === "cancel"
            ? t("services.eventsCancelConfirmTitle")
            : t("services.eventsRollbackConfirmTitle")
        }
        description={
          confirm === "cancel"
            ? t("services.eventsCancelConfirmBody")
            : t("services.eventsRollbackConfirmBody")
        }
        cancelLabel={t("services.eventsConfirmCancel")}
        confirmLabel={t("services.eventsConfirmProceed")}
        onConfirm={() => void handleConfirm()}
      />
    </>
  );
}
