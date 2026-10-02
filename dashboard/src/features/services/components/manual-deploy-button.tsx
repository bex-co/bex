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
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
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
 * does, so the next push cannot replace the pin. A specific commit also needs
 * the create grant because it chooses executable content.
 */
export function ManualDeployButton({
  service,
  pending,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const deployActions = useDeployActions(service.id);
  const actionConfirm = useBoundActionConfirm({ resourceId: service.id });
  const commitConfirm = useBoundActionConfirm({
    resourceId: service.id,
    requiredCapability: "can_create",
  });
  const { deploying, trigger, restart } = useTriggerDeploy();
  const { setAutoDeploy, busy: autoDeployBusy } = useAutoDeploy();
  const navigate = useNavigate();
  const [storedDialog, setDialog] = useState<{
    kind: "restart" | "commit";
    binding: ActionConfirmBinding;
    commitId: string;
  } | null>(null);
  const [checking, setChecking] = useState(false);
  const dialog =
    storedDialog?.binding ===
    (storedDialog?.kind === "commit"
      ? commitConfirm.pending
      : actionConfirm.pending)
      ? storedDialog
      : null;
  const commitId = dialog?.kind === "commit" ? dialog.commitId : "";
  const base = serviceBaseForType(service.type);
  const busy = checking || deploying || autoDeployBusy || pending;
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
  const commitReason =
    permissionReason ??
    (capabilities.canCreate
      ? undefined
      : t(
          capabilities.reasonKey("can_create") ?? "capabilities.actionChecking",
        ));

  function openDeploy(deployId: string | null) {
    if (!deployId) return;
    void navigate({
      to: `${base}/$serviceId/deploys/$deployId`,
      params: { serviceId: service.id, deployId },
    });
  }

  function openDialog(kind: "restart" | "commit") {
    if (busy || (kind === "commit" ? commitReason : permissionReason)) return;
    const confirmation = kind === "commit" ? commitConfirm : actionConfirm;
    const binding = confirmation.openConfirm("deploy");
    if (binding) setDialog({ kind, binding, commitId: "" });
  }

  function closeDialog() {
    actionConfirm.clearConfirm();
    commitConfirm.clearConfirm();
    setDialog(null);
  }

  async function runDeploy(
    confirmation: typeof actionConfirm,
    binding: ActionConfirmBinding | null,
    run: () => Promise<string | null>,
  ) {
    if (busy || !binding) return;
    setChecking(true);
    try {
      const { ok } = await confirmation.recheckBeforeDispatch(binding);
      if (!ok) return;
      const deployId = await run();
      if (confirmation.isBindingCurrent(binding)) openDeploy(deployId);
    } finally {
      confirmation.clearConfirm();
      setChecking(false);
    }
  }

  async function handleDeploy(opts?: { clearCache?: boolean }) {
    if (busy || permissionReason) return;
    await runDeploy(actionConfirm, actionConfirm.openConfirm("deploy"), () =>
      opts?.clearCache
        ? trigger(service.id, { clearCache: "clear" })
        : trigger(service.id),
    );
  }

  async function handleRestart() {
    if (permissionReason || dialog?.kind !== "restart") return;
    await runDeploy(actionConfirm, dialog.binding, () => restart(service.id));
  }

  async function handleDeployCommit() {
    if (commitReason || !shaValid || dialog?.kind !== "commit") return;
    const { binding } = dialog;
    await runDeploy(commitConfirm, binding, async () => {
      const deployId = await trigger(service.id, { commitId: sha });
      // Pinning also turns off auto-deploy; this second mutation needs its
      // own fresh check if access changed during the deploy request.
      if (deployId && service.autoDeploy !== false) {
        const { ok } = await commitConfirm.recheckBeforeDispatch(binding);
        if (ok) await setAutoDeploy(service.id, false);
      }
      return deployId;
    });
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
              permissionReason={commitReason}
              onSelect={() => openDialog("commit")}
            >
              {t("services.deployMenuSpecificCommit")}
            </PermissionMenuItem>
          )}
          {/* An image-backed service has no build, so no build cache to
              clear (w4/m141). */}
          {repoBacked && (
            <PermissionMenuItem
              disabled={busy}
              permissionReason={permissionReason}
              onSelect={() => void handleDeploy({ clearCache: true })}
            >
              {t("services.deployMenuClearCache")}
            </PermissionMenuItem>
          )}
          <DropdownMenuSeparator />
          <PermissionMenuItem
            disabled={busy}
            permissionReason={permissionReason}
            onSelect={() => openDialog("restart")}
          >
            {t("services.deployMenuRestart")}
          </PermissionMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={dialog?.kind === "restart"}
        onOpenChange={(open) => !open && closeDialog()}
        title={t("services.confirmRestartTitle", { name: service.name })}
        description={t(
          service.type === "static_site"
            ? "services.confirmRestartBodyStatic"
            : "services.confirmRestartBody",
          { name: service.name },
        )}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.actionRestart")}
        destructive={false}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!!permissionReason}
        onConfirm={() => void handleRestart()}
      />

      <ConfirmDialog
        open={dialog?.kind === "commit"}
        onOpenChange={(open) => !open && closeDialog()}
        title={t("services.deployCommitTitle")}
        description={t("services.deployCommitBody")}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.deployCommitConfirm")}
        destructive={false}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!shaValid || !!commitReason}
        onConfirm={() => void handleDeployCommit()}
      >
        <TextField
          id="deploy-commit-sha"
          label={t("services.deployCommitLabel")}
          value={commitId}
          onChange={(value) => {
            if (dialog) setDialog({ ...dialog, commitId: value });
          }}
          error={shaError ? t("services.deployCommitInvalid") : undefined}
        />
      </ConfirmDialog>
    </>
  );
}
