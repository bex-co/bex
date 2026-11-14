import type { ServiceView } from "@/features/services/types";

/**
 * Whether a service's Deploys page flags that auto-deploys are off, as
 * Render's does (changelog 2026-09-03: "A banner flagging when auto-deploys
 * are disabled", w3/064). Only a repo-backed service has auto-deploy to turn
 * off; an image-backed one deploys manually by design, and an unselected
 * field (null) is unknown, never off.
 */
export function autoDeployOff(
  service: Pick<ServiceView, "repo" | "autoDeploy"> | null | undefined,
): boolean {
  return !!service?.repo && service.autoDeploy === false;
}

/**
 * Whether no push to the service's repo can reach bex (pushDeliveryMethod
 * "unavailable"), so its deploys are manual for good, as Render's are for a
 * public Git URL ("must be deployed manually"): the banner then offers no
 * Auto-Deploy setting to turn on.
 */
export function deploysManualOnly(
  service: Pick<ServiceView, "pushDeliveryMethod"> | null | undefined,
): boolean {
  return service?.pushDeliveryMethod === "unavailable";
}
