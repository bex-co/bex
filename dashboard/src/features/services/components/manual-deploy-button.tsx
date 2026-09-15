import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ChevronDown } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { TextField } from "@/common/components/text-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAutoDeploy } from "@/features/services/hooks/use-auto-deploy";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import { isCron } from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";

/** A full or abbreviated Git commit SHA — the refs the dialog accepts. */
const COMMIT_SHA = /^[0-9a-f]{7,40}$/i;

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** Whether any action is already in flight for this service. */
  pending: boolean;
}

/**
 * Render's "Manual Deploy" header dropdown: "Deploy latest commit" (or
 * "Deploy latest image"), "Deploy a specific commit" for a repo-backed service,
 * "Clear build cache & deploy", a divider, then "Restart service".
 *
 * Every item opens a deploy-history row and lands on its page. Restart runs
 * `restartServer`, which keeps the commit or image that is live; a
 * parameter-free `triggerDeploy` would build the branch head (w1/m148). A
 * specific-commit deploy also turns auto-deploy off, as Render's dashboard
 * does, so the next push cannot replace the pin. Permission gates on the
 * deploy verb (w6/m143).
 */
export function ManualDeployButton({
  service,
  pending,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const deployActions = useDeployActions(service.id);
  const { deploying, trigger, restart } = useTriggerDeploy();
  const { setAutoDeploy, busy: autoDeployBusy } = useAutoDeploy();
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<"restart" | "commit" | null>(null);
  const [commitId, setCommitId] = useState("");
  const base = serviceBaseForType(service.type);
  const busy = deploying || autoDeployBusy || pending;
  const repoBacked = !!service.repo;
  // Render: a specific commit is "Not supported for cron jobs".
  const canDeployCommit = repoBacked && !isCron(service);
  const sha = commitId.trim();
  const shaValid = COMMIT_SHA.test(sha);
  const shaError = sha !== "" && !shaValid;

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

  function openDeploy(deployId: string | null) {
    if (!deployId) return;
    void navigate({
      to: `${base}/$serviceId/deploys/$deployId`,
      params: { serviceId: service.id, deployId },
    });
  }

  // Each handler rechecks permission right before dispatch so a stale allow
  // cannot fire after permission loss while the menu or dialog stayed open.
  async function handleDeploy(opts?: { clearCache?: boolean }) {
    if (permissionReason) return;
    await deployActions.refresh();
    openDeploy(
      opts?.clearCache
        ? await trigger(service.id, { clearCache: "clear" })
        : await trigger(service.id),
    );
  }

  async function handleRestart() {
    if (permissionReason) return;
    await deployActions.refresh();
    openDeploy(await restart(service.id));
  }

  async function handleDeployCommit() {
    if (permissionReason || !shaValid) return;
    await deployActions.refresh();
    const deployId = await trigger(service.id, { commitId: sha });
    if (!deployId) return;
    // The API's commitId leaves auto-deploy on; Render's dashboard turns it
    // off so the next push doesn't replace the commit just deployed.
    if (service.autoDeploy !== false) {
      await setAutoDeploy(service.id, false);
    }
    openDeploy(deployId);
  }

  return (
    <>
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
          {canDeployCommit && (
            <PermissionMenuItem
              disabled={busy}
              permissionReason={permissionReason}
              onSelect={() => setDialog("commit")}
            >
              {t("services.deployMenuSpecificCommit")}
            </PermissionMenuItem>
          )}
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
            onSelect={() => setDialog("restart")}
          >
            {t("services.deployMenuRestart")}
          </PermissionMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={dialog === "restart"}
        onOpenChange={(open) => !open && setDialog(null)}
        title={t("services.confirmRestartTitle", { name: service.name })}
        description={t("services.confirmRestartBody", { name: service.name })}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.actionRestart")}
        destructive={false}
        onConfirm={() => void handleRestart()}
      />

      <ConfirmDialog
        open={dialog === "commit"}
        onOpenChange={(open) => {
          if (open) return;
          setDialog(null);
          setCommitId("");
        }}
        title={t("services.deployCommitTitle")}
        description={t("services.deployCommitBody")}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.deployCommitConfirm")}
        destructive={false}
        confirmDisabled={!shaValid}
        onConfirm={() => void handleDeployCommit()}
      >
        <TextField
          id="deploy-commit-sha"
          label={t("services.deployCommitLabel")}
          value={commitId}
          onChange={setCommitId}
          error={shaError ? t("services.deployCommitInvalid") : undefined}
        />
      </ConfirmDialog>
    </>
  );
}
