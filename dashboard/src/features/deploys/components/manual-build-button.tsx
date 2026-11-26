import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { projectionReason } from "@/features/capabilities/lib/resource-actions";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { PipelineMinutesReason } from "@/features/usage/components/pipeline-minutes-reason";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { MANUAL_BUILD_BUTTON_CLASS } from "./deploys-list-frame";

/**
 * Render's Manual Build, beside a repo-backed cron job's deploy search
 * (w3/m555): it builds the branch's latest commit at once, with no menu or
 * confirm, and opens the new build with no toast; a failure toasts the
 * server's message until it is dismissed (w3/m643). A cron job's deploy is
 * its build, so this is the deploy trigger the header's Manual Deploy menu
 * used to hold for a cron job. Disabled, with the reason in a tooltip, while
 * the workspace's pipeline minutes are spent (Render's notice, ahead of any
 * other reason, as Render's `h_e` puts it, w3/108) or the server's `deploy`
 * decision refuses it.
 */
export function ManualBuildButton({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const deployActions = useDeployActions(serviceId);
  const { deploying, trigger } = useTriggerDeploy();
  const navigate = useNavigate();
  const [building, setBuilding] = useState(false);
  const busy = building || deploying;
  const minutesSpent = usePipelineMinutesExceeded();
  const reason = minutesSpent ? (
    <PipelineMinutesReason />
  ) : (
    projectionReason(deployActions, currentWorkspaceId, serviceId, "deploy", t)
  );

  async function build() {
    if (busy || reason) return;
    setBuilding(true);
    try {
      const deployId = await trigger(serviceId, { toastOnlyFailure: true });
      if (deployId) {
        await navigate({
          to: "/services/$serviceId/deploys/$deployId",
          params: { serviceId, deployId },
        });
      }
    } finally {
      setBuilding(false);
    }
  }

  return (
    <PermissionTooltip reason={reason}>
      <Button
        variant="outline"
        size="sm"
        className={MANUAL_BUILD_BUTTON_CLASS}
        disabled={busy || !!reason}
        onClick={() => void build()}
      >
        {busy ? <Loader2 className="animate-spin" /> : null}
        {t("deploys.manualBuild")}
      </Button>
    </PermissionTooltip>
  );
}
