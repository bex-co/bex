import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ChevronDown } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
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
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import type { ServiceView } from "@/features/services/types";

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** Whether any action is already in flight for this service. */
  pending: boolean;
}

type ConfirmAction = "deploy" | "restart";

/**
 * Render's "Manual Deploy" header dropdown ("Deploy latest commit" /
 * "Deploy latest image", a divider, then "Restart service").
 *
 * Both "Deploy" and "Restart service" route through the same `triggerDeploy`
 * mutation (w2/m30 consolidation) so every rollout — including a restart —
 * opens a deploy-history row in the Events tab.
 *
 * For repo-backed services "Restart service" triggers a rebuild from Branch
 * HEAD (bex has no way to restart pods without a new build — any spec change
 * increments the generation and unconditionally re-enters the build path).
 * For image-backed services it re-pulls and restarts the containers in place.
 *
 * "Deploy a specific commit" and "Clear build cache & deploy" from Render's
 * dropdown are omitted: bex's builds are always cache-free (ephemeral BuildKit
 * Jobs), and per-commit targeting via commitId is an API-only feature for now.
 */
export function ManualDeployButton({ service, pending }: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { deploying, trigger } = useTriggerDeploy();
  const navigate = useNavigate();
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  const busy = deploying || pending;
  const repoBacked = !!service.repo;

  const deployLabel = repoBacked
    ? t("services.deployMenuLatestCommit")
    : t("services.deployMenuLatestImage");

  const confirmTitle =
    confirm === "restart"
      ? t("services.confirmRestartTitle", { name: service.name })
      : repoBacked
        ? t("services.deployConfirmCommitTitle", {
            branch: service.branch ?? "",
          })
        : t("services.deployConfirmImageTitle", { name: service.name });

  const confirmBody =
    confirm === "restart"
      ? t("services.confirmRestartBody")
      : repoBacked
        ? t("services.deployConfirmCommitBody", {
            name: service.name,
            branch: service.branch ?? "",
          })
        : t("services.deployConfirmImageBody", { name: service.name });

  async function handleConfirm() {
    // Both "Deploy" and "Restart" go through triggerDeploy — the same mutation
    // — so both open a deploy-history row (w2/m30). Restart passes no extra
    // options: for image-backed services this re-pulls and restarts; for
    // repo-backed services this rebuilds from Branch HEAD.
    const deployId = await trigger(service.id);
    setConfirm(null);
    // Render lands the user straight on the new deploy's page (w9/m1/t004); a
    // failed trigger already toasted and has no deploy id to navigate to.
    if (deployId) {
      void navigate({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: service.id, deployId },
      });
    }
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
          <DropdownMenuItem disabled={busy} onSelect={() => setConfirm("deploy")}>
            {deployLabel}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            disabled={busy}
            onSelect={() => setConfirm("restart")}
          >
            {t("services.deployMenuRestart")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog
        open={confirm !== null}
        onOpenChange={(open) => !open && setConfirm(null)}
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
              onClick={(e) => {
                e.preventDefault();
                void handleConfirm();
              }}
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
