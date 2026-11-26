// Render's Scaling fact on a deploy page (w1/m661, docs/render-artifacts/
// deploy-detail-page.md § Scaling): how many of the service's instances run
// this deploy, out of how many it needs.

import { isCancelableDeployStatus } from "./deploy-status";

/** One of the service's instances, as GraphQL `serviceInstances` names it. */
export interface ScalingInstance {
  /** The deploy it runs; null when no deploy row records its release. */
  deployId: string | null;
  /** pending | running | succeeded | failed | unknown, its pod's phase. */
  status: string | null;
}

/** The service's autoscaling, as GraphQL `autoscalingConfig` answers it. */
export interface ScalingAutoscaling {
  enabled: boolean | null;
  minInstances: number | null;
}

/** "`deployed` of `total` instances". */
export interface DeployScaling {
  deployed: number;
  total: number;
}

/** A service Render counts instances for: one that autoscales or runs more
 *  than one instance. */
export function scalesOut(
  replicas: number | null,
  autoscaling: ScalingAutoscaling | null | undefined,
): boolean {
  return autoscaling?.enabled === true || (replicas ?? 1) > 1;
}

/**
 * Render's count (`index`'s `P$`): the instances running this deploy (status
 * running or succeeded), out of the autoscaling minimum when the service
 * autoscales or its instance count otherwise, never fewer in all than run
 * this deploy. Null for a service that does not scale out, or while its
 * instances have not loaded.
 */
export function deployScaling({
  deployId,
  replicas,
  autoscaling,
  instances,
}: {
  deployId: string;
  replicas: number | null;
  autoscaling: ScalingAutoscaling | null | undefined;
  instances: readonly ScalingInstance[] | null | undefined;
}): DeployScaling | null {
  if (!instances || !scalesOut(replicas, autoscaling)) return null;
  const deployed = instances.filter(
    (i) =>
      i.deployId === deployId &&
      (i.status === "running" || i.status === "succeeded"),
  ).length;
  const needed = autoscaling?.enabled
    ? (autoscaling.minInstances ?? 1)
    : (replicas ?? 1);
  return { deployed, total: Math.max(deployed, needed) };
}

/** Render's in-progress deploy (`CREATED`, `BUILDING`, `PREDEPLOY`,
 *  `IN_PROGRESS`): open and past its queue. */
export function isInProgressDeployStatus(status: string): boolean {
  return status !== "queued" && isCancelableDeployStatus(status);
}

/**
 * When Render's deploy header carries the fact (`useDeployDetails`): on a
 * live, queued or in-progress deploy, or one the page saw in progress, so a
 * rollout watched until it fails or is canceled keeps its last count.
 */
export function scalingShows(
  status: string,
  watchedInProgress: boolean,
): boolean {
  return (
    status === "live" || isCancelableDeployStatus(status) || watchedInProgress
  );
}
