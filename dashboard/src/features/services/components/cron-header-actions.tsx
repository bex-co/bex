import { Link } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import {
  projectionReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";
import { useTriggerCronRun } from "@/features/services/hooks/use-trigger-cron-run";
import { hasServed } from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** A slot a translated sentence carries, so a link or code can sit where
 *  each language puts it: translate with the slot as the value, then split. */
export const SLOT = "\u0000";

/**
 * A running cron job's one header action, Render's Trigger Run (w3/m555): a
 * click runs it at once and opens its Logs on Live tail. Disabled, with the
 * reason in a tooltip, while the server's `cron_run_now` decision refuses it
 * (no permission, suspended, a billing hold) and, for a repo-backed cron job,
 * until a build has gone live: there is nothing to run before that. An
 * image-backed cron job runs its image, so it has no such wait.
 */
export function CronTriggerRunButton({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(service.id);
  const { triggering, trigger } = useTriggerCronRun(service.id);
  const blocked = projectionReason(
    serverActions,
    currentWorkspaceId,
    service.id,
    "cron_run_now",
    t,
  );
  // Render names the suspended case in its own words, after a missing
  // permission (w3/m556): the button shows while suspended only in the Runs
  // page's empty states.
  const decision =
    serverActions.status === "ready"
      ? resourceDecision(
          serverActions.snapshot,
          currentWorkspaceId,
          service.id,
          "cron_run_now",
        )
      : null;
  const suspended =
    decision?.outcome === "allowed" && decision.precondition === "suspended";
  const noBuild = Boolean(service.repo) && !hasServed(service);
  const reason = suspended
    ? t("services.cronTriggerSuspended")
    : (blocked ?? (noBuild ? <NoBuildReason serviceId={service.id} /> : null));

  return (
    <PermissionTooltip reason={reason}>
      <Button
        size="sm"
        disabled={triggering || reason !== null}
        onClick={() => void trigger()}
      >
        {triggering ? <Loader2 className="animate-spin" /> : null}
        {t("services.cronTriggerRun")}
      </Button>
    </PermissionTooltip>
  );
}

/** "No successful build detected. View Deploys.", Deploys linked. */
function NoBuildReason({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const [before, after = ""] = t("services.cronTriggerNoBuild", {
    deploys: SLOT,
  }).split(SLOT);
  return (
    <>
      {before}
      <Link
        to="/services/$serviceId/deploys"
        params={{ serviceId }}
        className="underline underline-offset-2"
      >
        {t("services.navDeploys")}
      </Link>
      {after}
    </>
  );
}
