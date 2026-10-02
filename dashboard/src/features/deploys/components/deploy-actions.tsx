import { useCallback, useState } from "react";
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
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useServiceBase } from "@/features/services/lib/service-base";
import {
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { deployCommitLabel } from "@/features/deploys/lib/deploy-presentation";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import {
  useDeployActions,
  type ResourceActionsState,
} from "@/features/capabilities/hooks/use-resource-actions";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  decisionForSelectedRollback,
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";

type ConfirmAction = "cancel" | "rollback";

export interface DeployActionsProps {
  serviceId: string;
  deployId: string;
  status: string;
  /**
   * The selected deploy's commit, when the caller has it. Present => the
   * rollback confirm dialog names `<short-sha> <subject>`; absent or
   * unresolvable => it keeps the generic body (w4/m110 t003).
   */
  commitId?: string | null;
  commitMessage?: string | null;
  /**
   * The selected deploy's trigger (`create`, `api`, `deploy_hook`, …). It is
   * used for exactly one thing: a `create` deploy is the service's FIRST build,
   * so canceling it leaves nothing serving — and the cancel confirm must say so
   * instead of promising that "the last successful deploy remains live", which
   * is false precisely when the user is deciding whether cancel strands them
   * (w4/103). Absent => the generic body, which is the safe default.
   */
  trigger?: string | null;
  onChanged?: () => void;
  /** A list shares its service-wide projection across all deploy rows. */
  projection?: ResourceActionsState;
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
  commitId,
  commitMessage,
  trigger,
  onChanged,
  projection,
}: DeployActionsProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const base = useServiceBase();
  const { currentWorkspaceId } = useWorkspace();
  const ownProjection = useDeployActions(projection ? null : serviceId);
  const deployActions = projection ?? ownProjection;
  const statusCancel = isCancelableDeployStatus(status);
  const statusRollback = isRollbackableDeployStatus(status);

  // The selected row's own status outranks the service-wide summary: a
  // rollbackable row stays a target when the latest-20 scan found none
  // (w6/m143/t003), and a cancelable row is open whatever a stale
  // no_active_deploy says. One rule for enabling and for the dispatch
  // recheck, which used the raw summary and silently refused (w4/m141).
  const rowDecision = useCallback(
    (
      action: ResourceActionId,
      decision: ResourceActionDecision | null,
    ): ResourceActionDecision | null => {
      if (action === "rollback" && statusRollback) {
        return decisionForSelectedRollback(decision);
      }
      if (
        action === "cancel_deploy" &&
        statusCancel &&
        decision?.outcome === "allowed" &&
        decision.precondition === "no_active_deploy"
      ) {
        return { ...decision, precondition: "" };
      }
      return decision;
    },
    [statusRollback, statusCancel],
  );
  const {
    pending,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
    isBindingCurrent,
  } = useBoundActionConfirm({
    resourceId: serviceId,
    deployId,
    adjustDecision: rowDecision,
  });
  const [cancelDeploy, { loading: canceling }] = useMutation(
    CancelDeployDocument,
    { refetchQueries: DEPLOY_REFETCH_QUERIES },
  );
  const [rollbackService, { loading: rollingBack }] = useMutation(
    RollbackServiceDocument,
    { refetchQueries: DEPLOY_REFETCH_QUERIES },
  );
  const [checking, setChecking] = useState(false);
  const busy = checking || canceling || rollingBack;

  function reasonFor(action: "cancel_deploy" | "rollback"): string | undefined {
    if (deployActions.status !== "ready") {
      return gateReason(gateAction(null, deployActions.status), t);
    }
    const decision = rowDecision(
      action,
      resourceDecision(
        deployActions.snapshot,
        currentWorkspaceId,
        serviceId,
        action,
      ),
    );
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
    if (busy || !pending) return;
    const action = pending.action;
    setChecking(true);
    try {
      const { ok, binding } = await recheckBeforeDispatch();
      if (!ok || !binding) return;
      if (action === "cancel_deploy") {
        await cancelDeploy({
          variables: {
            serviceId: binding.resourceId,
            deployId: binding.deployId ?? deployId,
          },
        });
        if (!isBindingCurrent(binding)) return;
        toast.success(t("services.cancelDeploySuccess"));
      } else if (action === "rollback") {
        const { data } = await rollbackService({
          variables: {
            serviceId: binding.resourceId,
            deployId: binding.deployId ?? deployId,
          },
        });
        if (!isBindingCurrent(binding)) return;
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
    } catch (err) {
      toast.error(
        mutationErrorMessage(
          err,
          action === "cancel_deploy"
            ? t("services.cancelDeployError")
            : t("services.rollbackError"),
        ),
      );
    } finally {
      clearConfirm();
      setChecking(false);
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

  // Name the code the rollback restores; a commit-less deploy keeps the
  // generic body rather than naming nothing (w4/m110 t003). A static site is
  // re-published, with no image or instances to speak of (w4/m141).
  const commit = deployCommitLabel(commitId, commitMessage);
  const staticSite = base === "/static";
  const rollbackBody = staticSite
    ? commit
      ? t("services.eventsRollbackConfirmBodyStaticCommit", { commit })
      : t("services.eventsRollbackConfirmBodyStatic")
    : commit
      ? t("services.eventsRollbackConfirmBodyCommit", { commit })
      : t("services.eventsRollbackConfirmBody");
  // Each row's controls name their deploy, so two rows are not two identical
  // "Rollback" buttons to a screen reader (w4/m141, as w4/084 did for revoke).
  const shortSha = (commitId ?? "").trim().slice(0, 7);
  const target = shortSha ? `${shortSha} (${deployId})` : deployId;

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
              aria-label={t("services.eventsCancelDeployAria", { target })}
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
              aria-label={t("services.eventsRollbackAria", { target })}
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
            ? t(
                trigger === "create"
                  ? "services.eventsCancelConfirmBodyFirstDeploy"
                  : "services.eventsCancelConfirmBody",
              )
            : rollbackBody
        }
        cancelLabel={t("services.eventsConfirmCancel")}
        confirmLabel={t("services.eventsConfirmProceed")}
        pending={busy}
        closeOnConfirm={false}
        onConfirm={() => void handleConfirm()}
      />
    </>
  );
}
