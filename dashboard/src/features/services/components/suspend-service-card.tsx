import { Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { publiclyRoutable } from "@/features/services/lib/service-type";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { useState } from "react";
import type { en } from "@/i18n";

const ACTION_LABEL: Record<"suspend" | "resume", keyof typeof en> = {
  suspend: "services.actionSuspend",
  resume: "services.actionResume",
};

export interface SuspendServiceCardProps {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}

/**
 * Settings-tab suspend/resume card: mirrors the bottom-of-settings pattern
 * from Render's own dashboard. Shows "Suspend Service" when live and
 * "Resume Service" when suspended. Gated by serverActions (w6/m143).
 */
export function SuspendServiceCard({
  service,
  pending,
  onRun,
}: SuspendServiceCardProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(service.id);
  const { pending: confirmBinding, openConfirm, clearConfirm, recheckBeforeDispatch } =
    useBoundActionConfirm({ resourceId: service.id });
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: LifecycleAction;
    confirmation: string;
  } | null>(null);
  const busy = pending !== null;
  const isSuspended = service.suspended;
  const hasUrl = publiclyRoutable(service.type);
  const actionId: ResourceActionId = isSuspended ? "resume" : "suspend";

  const decision =
    serverActions.status === "ready"
      ? resourceDecision(
          serverActions.snapshot,
          currentWorkspaceId,
          service.id,
          actionId,
        )
      : null;
  const gate = gateAction(
    decision,
    serverActions.status === "ready" ? "ready" : serverActions.status,
  );
  const permissionReason = gateReason(gate, t);
  const confirmOpen = confirmBinding?.action === "suspend";

  async function runAction(action: LifecycleAction, confirmation?: string) {
    if (permissionReason && !confirmation) return;
    const result = confirmation
      ? await onRun(action, service, confirmation)
      : await onRun(action, service);
    if (result.status === "confirmation_required") {
      setProtectedConfirm({ action, confirmation: result.confirmation });
    } else if (result.status === "success") {
      setProtectedConfirm(null);
    }
  }

  async function handleResume() {
    if (permissionReason) return;
    await runAction("resume");
  }

  async function handleSuspendConfirm() {
    const { ok } = await recheckBeforeDispatch();
    if (!ok) return;
    clearConfirm();
    await runAction("suspend");
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {t(
            isSuspended
              ? "services.resumeCardTitle"
              : "services.suspendCardTitle",
          )}
        </CardTitle>
        <CardDescription>
          {t(
            isSuspended
              ? "services.resumeCardDescription"
              : hasUrl
                ? "services.suspendCardDescription"
                : "services.suspendCardDescriptionNoUrl",
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isSuspended ? (
          <PermissionTooltip reason={permissionReason}>
            <Button
              onClick={() => void handleResume()}
              disabled={busy || !!permissionReason}
            >
              {busy && pending === "resume" ? (
                <Loader2 className="animate-spin" />
              ) : null}
              {t("services.actionResume")}
            </Button>
          </PermissionTooltip>
        ) : (
          <PermissionTooltip reason={permissionReason}>
            <Button
              variant="outline"
              onClick={() => {
                if (permissionReason) return;
                openConfirm("suspend");
              }}
              disabled={busy || !!permissionReason}
            >
              {busy && pending === "suspend" ? (
                <Loader2 className="animate-spin" />
              ) : null}
              {t("services.actionSuspend")}
            </Button>
          </PermissionTooltip>
        )}
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && clearConfirm()}
        title={t("services.confirmSuspendTitle", { name: service.name })}
        description={t(
          hasUrl
            ? "services.confirmSuspendBody"
            : "services.confirmSuspendBodyNoUrl",
          { name: service.name },
        )}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.actionSuspend")}
        onConfirm={() => void handleSuspendConfirm()}
      />

      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={protectedConfirm !== null}
        resourceName={service.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={
          protectedConfirm &&
          (protectedConfirm.action === "suspend" ||
            protectedConfirm.action === "resume")
            ? t(ACTION_LABEL[protectedConfirm.action])
            : ""
        }
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirm(null)}
        onConfirm={(confirmation) =>
          protectedConfirm
            ? runAction(protectedConfirm.action, confirmation)
            : Promise.resolve()
        }
      />
    </Card>
  );
}
