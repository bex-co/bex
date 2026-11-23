import { Link } from "@tanstack/react-router";
import { Loader2, PauseCircle } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import { projectionReason } from "@/features/capabilities/lib/resource-actions";
import { ResumeServiceButton } from "@/features/services/components/resume-service-button";
import { useTriggerCronRun } from "@/features/services/hooks/use-trigger-cron-run";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { hasServed } from "@/features/services/lib/status";
import type { LifecycleAction, ServiceView } from "@/features/services/types";
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
  const noBuild = Boolean(service.repo) && !hasServed(service);
  const reason =
    blocked ?? (noBuild ? <NoBuildReason serviceId={service.id} /> : null);

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

/** The notice's box, which its loading frame draws too. */
export const CRON_SUSPENDED_NOTICE_CLASS =
  "bg-muted/40 flex min-h-11 flex-wrap items-center gap-x-2 gap-y-0.5 rounded-md border px-3 py-1.5 text-sm";

/**
 * Render's notice under a suspended cron job's facts, which stands in for the
 * header's actions while it is suspended (w3/m555): "Manually suspended" when
 * its owner suspended it, "Suspended" when the platform did, that a suspended
 * service is not billed, and Resume service, gated by the server's `resume`
 * decision like every other Resume.
 */
export function CronSuspendedNotice({
  service,
  pending,
  onRun,
}: {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun?: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="status"
      data-cron-suspended-notice=""
      className={CRON_SUSPENDED_NOTICE_CLASS}
    >
      <PauseCircle
        aria-hidden="true"
        className="text-muted-foreground size-4 shrink-0"
      />
      <span className="font-medium">
        {t(
          service.manuallySuspended
            ? "services.suspendedNoticeManual"
            : "services.suspendedNoticeTitle",
        )}
      </span>{" "}
      <span className="text-muted-foreground">
        {t("services.suspendedNoticeBody")}
      </span>{" "}
      {onRun ? (
        <ResumeServiceButton
          service={service}
          pending={pending}
          onRun={onRun}
          size="sm"
          variant="link"
          label={t("services.suspendedNoticeResume")}
          className="h-8 px-0"
        />
      ) : null}
    </div>
  );
}
