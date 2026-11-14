import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { ConfirmGateStatus } from "@/features/capabilities/components/confirm-gate-status";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import {
  useBoundActionConfirm,
  type ActionConfirmBinding,
} from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";

export interface ResumeServiceButtonProps {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
  size?: "default" | "sm";
}

/**
 * Resume Service, gated by the service's `resume` action decision (w6/m143)
 * and rechecked before dispatch (w4/m159). A protected environment answers
 * with the phrase to retry with. Shared by the Settings card and the service
 * header, which offers it while the service is suspended (w4/643).
 */
export function ResumeServiceButton({
  service,
  pending,
  onRun,
  size,
}: ResumeServiceButtonProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(service.id);
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

  const decision =
    serverActions.status === "ready"
      ? resourceDecision(
          serverActions.snapshot,
          currentWorkspaceId,
          service.id,
          "resume",
        )
      : null;
  const permissionReason = gateReason(
    gateAction(
      decision,
      serverActions.status === "ready" ? "ready" : serverActions.status,
    ),
    t,
  );

  function close() {
    clearConfirm();
    setProtectedConfirm(null);
  }

  async function run(
    binding: ActionConfirmBinding | null,
    confirmation?: string,
  ) {
    if (!binding || busy || binding.action !== "resume") return;
    setChecking(true);
    try {
      const { ok } = await recheckBeforeDispatch(binding);
      if (!ok) return;
      const result = confirmation
        ? await onRun("resume", service, confirmation)
        : await onRun("resume", service);
      // The mutation already ran: follow its result unless the context
      // changed. A protected re-prompt still rechecks before it dispatches.
      if (!isIntentCurrent(binding)) return;
      if (result.status === "confirmation_required") {
        setProtectedConfirm({ binding, confirmation: result.confirmation });
      } else if (result.status === "success" || !confirmation) {
        close();
      }
    } finally {
      setChecking(false);
    }
  }

  return (
    <>
      <PermissionTooltip reason={permissionReason}>
        <Button
          size={size}
          onClick={() => {
            if (busy || permissionReason) return;
            void run(openConfirm("resume"));
          }}
          disabled={busy || !!permissionReason}
        >
          {busy && pending === "resume" ? (
            <Loader2 className="animate-spin" />
          ) : null}
          {t("services.actionResume")}
        </Button>
      </PermissionTooltip>
      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={protectedConfirm !== null}
        resourceName={service.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={t("services.actionResume")}
        busy={busy}
        confirmDisabled={!!blockedReason}
        onOpenChange={(open) => !open && close()}
        onConfirm={(confirmation) =>
          protectedConfirm
            ? run(protectedConfirm.binding, confirmation)
            : Promise.resolve()
        }
      >
        <ConfirmGateStatus reason={blockedReason} />
      </ProtectedConfirmationDialog>
    </>
  );
}
