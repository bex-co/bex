import { useNavigate } from "@tanstack/react-router";
import { ChevronDown } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import type { ServiceView } from "@/features/services/types";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** Whether any action is already in flight for this service. */
  pending: boolean;
}

/**
 * Render's "Manual Deploy" header dropdown ("Deploy latest commit" /
 * "Deploy latest image", a divider, then "Restart service").
 *
 * Both "Deploy" and "Restart service" route through the same `triggerDeploy`
 * mutation (w2/m30 consolidation) so every rollout — including a restart —
 * opens a deploy-history row in the Events tab. Permission gates on the
 * deploy verb (w6/m143).
 */
export function ManualDeployButton({
  service,
  pending,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const deployActions = useDeployActions(service.id);
  const { deploying, trigger } = useTriggerDeploy();
  const navigate = useNavigate();
  const base = serviceBaseForType(service.type);
  const busy = deploying || pending;
  const repoBacked = !!service.repo;

  const deployLabel = repoBacked
    ? t("services.deployMenuLatestCommit")
    : t("services.deployMenuLatestImage");

  const decision =
    deployActions.status === "ready"
      ? resourceDecision(
          deployActions.snapshot,
          currentWorkspaceId,
          service.id,
          "deploy",
        )
      : null;
  const gate = gateAction(
    decision,
    deployActions.status === "ready" ? "ready" : deployActions.status,
  );
  const permissionReason = gateReason(gate, t);

  async function handleDeploy(opts?: { clearCache?: boolean }) {
    if (permissionReason) return;
    // Fresh recheck before dispatch so a stale allow cannot fire after
    // permission loss while the menu stayed open.
    await deployActions.refresh();
    const deployId = opts?.clearCache
      ? await trigger(service.id, { clearCache: "clear" })
      : await trigger(service.id);
    if (deployId) {
      void navigate({
        to: `${base}/$serviceId/deploys/$deployId`,
        params: { serviceId: service.id, deployId },
      });
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" disabled={busy}>
          {t("services.eventsManualDeploy")}
          <ChevronDown className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <PermissionMenuItem
          disabled={busy}
          permissionReason={permissionReason}
          onSelect={() => void handleDeploy()}
        >
          {deployLabel}
        </PermissionMenuItem>
        <PermissionMenuItem
          disabled={busy}
          permissionReason={permissionReason}
          onSelect={() => void handleDeploy({ clearCache: true })}
        >
          {t("services.deployMenuClearCache")}
        </PermissionMenuItem>
        <DropdownMenuSeparator />
        <PermissionMenuItem
          disabled={busy}
          permissionReason={permissionReason}
          onSelect={() => void handleDeploy()}
        >
          {t("services.deployMenuRestart")}
        </PermissionMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
