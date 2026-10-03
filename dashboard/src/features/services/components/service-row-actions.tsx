import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { isCron, publiclyRoutable } from "@/features/services/lib/service-type";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import { ConfirmGateStatus } from "@/features/capabilities/components/confirm-gate-status";
import {
  useDeployActions,
  useServerActions,
} from "@/features/capabilities/hooks/use-resource-actions";
import {
  useBoundActionConfirm,
  type ActionConfirmBinding,
} from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";

const ACTION_LABEL: Record<LifecycleAction, keyof typeof en> = {
  suspend: "services.actionSuspend",
  resume: "services.actionResume",
  restart: "services.actionRestart",
};

// UI "restart" runs restartServer, which opens a deploy under the same
// authorization as triggerDeploy — gate on the deploy verb.
function decisionActionFor(action: LifecycleAction): ResourceActionId {
  return action === "restart" ? "deploy" : action;
}

function lifecycleActionFor(action: ResourceActionId): LifecycleAction | null {
  if (action === "deploy") return "restart";
  return action === "suspend" || action === "resume" ? action : null;
}

const CONFIRM: Partial<
  Record<LifecycleAction, { title: keyof typeof en; body: keyof typeof en }>
> = {
  suspend: {
    title: "services.confirmSuspendTitle",
    body: "services.confirmSuspendBody",
  },
  restart: {
    title: "services.confirmRestartTitle",
    body: "services.confirmRestartBody",
  },
};

function confirmBodyKey(
  action: LifecycleAction,
  service: ServiceView,
): keyof typeof en {
  const body = CONFIRM[action]!.body;
  if (action === "suspend" && isCron(service)) {
    return "services.suspendCronDescription";
  }
  if (
    body === "services.confirmRestartBody" &&
    service.type === "static_site"
  ) {
    return "services.confirmRestartBodyStatic";
  }
  return body === "services.confirmSuspendBody" &&
    !publiclyRoutable(service.type)
    ? "services.confirmSuspendBodyNoUrl"
    : body;
}

export interface ServiceRowActionsProps {
  service: ServiceView;
  /** The action in flight for this row, or null. Disables the control. */
  pending: LifecycleAction | null;
  onRun: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
  /**
   * Omit "Restart" from this menu (Render parity, service-detail header
   * only): the header's `ManualDeployButton` dropdown already carries
   * "Restart service" grouped with the deploy verbs, Render's own placement
   * — showing it here too would offer the same action under two different
   * menus. The services list still gets the full set; only the header opts in.
   */
  hideRestart?: boolean;
  /**
   * Omit "Suspend" and "Resume" from this menu (service-detail header only):
   * the settings page surfaces both as a dedicated card so they aren't
   * offered in two places simultaneously.
   */
  hideSuspend?: boolean;
}

export function ServiceRowActions({
  service,
  pending,
  onRun,
  hideRestart = false,
  hideSuspend = false,
}: ServiceRowActionsProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(service.id);
  const deployActions = useDeployActions(service.id);
  const {
    pending: confirmBinding,
    blockedReason,
    openConfirm,
    clearConfirm,
    recheckBeforeDispatch,
    isIntentCurrent,
  } = useBoundActionConfirm({ resourceId: service.id });
  const [storedProtectedConfirm, setProtectedConfirm] = useState<{
    binding: ActionConfirmBinding;
    confirmation: string;
  } | null>(null);
  const protectedConfirm =
    storedProtectedConfirm?.binding === confirmBinding
      ? storedProtectedConfirm
      : null;
  const [checking, setChecking] = useState(false);
  const busy = pending !== null || checking;

  // Render: restart is "Not supported for cron jobs" (w8/040).
  const noRestart = hideRestart || isCron(service);
  const actions: LifecycleAction[] = hideSuspend
    ? noRestart
      ? []
      : ["restart"]
    : service.suspended
      ? ["resume"]
      : noRestart
        ? ["suspend"]
        : ["suspend", "restart"];

  const boundAction = confirmBinding
    ? lifecycleActionFor(confirmBinding.action)
    : null;
  const confirmAction =
    !protectedConfirm && boundAction && CONFIRM[boundAction]
      ? boundAction
      : null;

  function reasonFor(action: LifecycleAction): string | undefined {
    const decisionId = decisionActionFor(action);
    const state = action === "restart" ? deployActions : serverActions;
    const decision =
      state.status === "ready"
        ? resourceDecision(
            state.snapshot,
            currentWorkspaceId,
            service.id,
            decisionId,
          )
        : null;
    const gate = gateAction(
      decision,
      state.status === "ready" ? "ready" : state.status,
    );
    return gateReason(gate, t);
  }

  // An open confirmation survives a routine permission refresh but cannot
  // dispatch until access is fresh again (w4/m159).
  const confirmBlocked =
    blockedReason ?? (confirmAction ? reasonFor(confirmAction) : undefined);

  function handleSelect(action: LifecycleAction) {
    if (busy || reasonFor(action)) return;
    if (CONFIRM[action]) {
      openConfirm(decisionActionFor(action));
    } else {
      void runAction(openConfirm(decisionActionFor(action)));
    }
  }

  function closeConfirm() {
    clearConfirm();
    setProtectedConfirm(null);
  }

  async function runAction(
    binding: ActionConfirmBinding | null,
    confirmation?: string,
  ) {
    const action = binding ? lifecycleActionFor(binding.action) : null;
    if (!binding || !action || busy) return;
    setChecking(true);
    try {
      const { ok } = await recheckBeforeDispatch(binding);
      if (!ok) return;
      const result = confirmation
        ? await onRun(action, service, confirmation)
        : await onRun(action, service);
      // The mutation already ran: follow its result unless the context
      // changed. A protected re-prompt still rechecks before it dispatches.
      if (!isIntentCurrent(binding)) return;
      if (result.status === "confirmation_required") {
        setProtectedConfirm({ binding, confirmation: result.confirmation });
      } else if (result.status === "success" || !confirmation) {
        closeConfirm();
      }
    } finally {
      setChecking(false);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("services.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {actions.map((action) => {
            const permissionReason = reasonFor(action);
            return (
              <PermissionMenuItem
                key={action}
                disabled={busy}
                permissionReason={permissionReason}
                variant={action === "suspend" ? "destructive" : "default"}
                onSelect={() => handleSelect(action)}
              >
                {t(ACTION_LABEL[action])}
              </PermissionMenuItem>
            );
          })}
          <MoveToProjectMenu
            kind="service"
            resourceId={service.id}
            resourceName={service.name}
            disabled={busy}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={confirmAction !== null}
        onOpenChange={(open) => !open && closeConfirm()}
        title={
          confirmAction
            ? t(CONFIRM[confirmAction]!.title, { name: service.name })
            : ""
        }
        description={
          confirmAction
            ? t(confirmBodyKey(confirmAction, service), {
                name: service.name,
              })
            : ""
        }
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={confirmAction ? t(ACTION_LABEL[confirmAction]) : ""}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!!confirmBlocked}
        onConfirm={() => {
          void runAction(confirmBinding);
        }}
      >
        <ConfirmGateStatus reason={confirmBlocked} />
      </ConfirmDialog>

      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={protectedConfirm !== null}
        resourceName={service.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={
          protectedConfirm && boundAction ? t(ACTION_LABEL[boundAction]) : ""
        }
        busy={busy}
        confirmDisabled={!!blockedReason}
        onOpenChange={(open) => !open && closeConfirm()}
        onConfirm={(confirmation) =>
          protectedConfirm
            ? runAction(protectedConfirm.binding, confirmation)
            : Promise.resolve()
        }
      >
        <ConfirmGateStatus reason={blockedReason} />
      </ProtectedConfirmationDialog>
    </>
  );
}
