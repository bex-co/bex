import { useEffect, useRef, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  AutoscalingConfigDocument,
  DeployInstancesDocument,
} from "@/graphql/definitions";
import {
  CONVERGING_POLL_INTERVAL_MS,
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import type { ServiceView } from "@/features/services/types";
import { supportsScaling } from "@/features/services/lib/service-type";
import {
  deployScaling,
  isInProgressDeployStatus,
  scalesOut,
  scalingShows,
  type DeployScaling,
  type ScalingInstance,
} from "../lib/deploy-scaling";
import { isCancelableDeployStatus } from "../lib/deploy-status";
import type { DeployView } from "./use-deploy";

/**
 * The deploy header's Scaling fact (w1/m661): the count, "loading" while the
 * instances of a service that scales out are still on their way (so the
 * header keeps the fact's place), or null when the fact does not show.
 */
export type DeployScalingFact = DeployScaling | "loading" | null;

/**
 * Reads what Render's deploy header counts its Scaling fact from: the
 * service's autoscaling and, only when it autoscales or runs more than one
 * instance, its instances with the deploy each runs. Only a deploy the fact
 * shows on reads them, every 3 seconds while the deploy is queued or in
 * progress and at the page's baseline once it is not, with one read as the
 * rollout ends so the count it ended on shows at once. The route loader warms
 * the autoscaling read (deploy-page-prefetch.ts), so the page mounts knowing
 * whether the fact shows rather than growing by it.
 */
export function useDeployScaling(
  serviceId: string,
  service: ServiceView | null,
  deploy: DeployView | null,
): DeployScalingFact {
  const status = deploy?.status ?? "";
  const deployId = deploy?.id ?? "";
  // A deploy the page saw in progress keeps its count after it ends.
  const [watched, setWatched] = useState<string | null>(null);
  if (deployId && isInProgressDeployStatus(status) && watched !== deployId) {
    setWatched(deployId);
  }
  const shows =
    deployId !== "" &&
    service !== null &&
    supportsScaling(service) &&
    scalingShows(status, watched === deployId);

  const { data: autoscalingData } = useQuery(AutoscalingConfigDocument, {
    variables: { id: serviceId },
    skip: !shows,
  });
  const autoscaling = autoscalingData?.autoscalingConfig;
  const replicas = service?.replicas ?? null;
  const readsInstances = shows && scalesOut(replicas, autoscaling);

  const rolling = isCancelableDeployStatus(status);
  const interval = rolling
    ? CONVERGING_POLL_INTERVAL_MS
    : RESOURCE_POLL_INTERVAL_MS;
  const { data, previousData, error, startPolling, refetch } = useQuery(
    DeployInstancesDocument,
    {
      variables: { id: serviceId },
      skip: !readsInstances,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      pollInterval: interval,
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  useEffect(() => {
    if (readsInstances) startPolling(interval);
  }, [readsInstances, interval, startPolling]);
  const wasRolling = useRef(rolling);
  useEffect(() => {
    if (wasRolling.current && !rolling && readsInstances) void refetch();
    wasRolling.current = rolling;
  }, [rolling, readsInstances, refetch]);

  if (!readsInstances) return null;
  const listed = (data ?? previousData)?.serviceInstances;
  // A first read that failed has no count to keep a place for.
  if (!listed) return error ? null : "loading";
  const instances: ScalingInstance[] = listed
    .filter((i): i is NonNullable<typeof i> => Boolean(i?.id))
    .map((i) => ({ deployId: i.deployId, status: i.status }));
  return deployScaling({ deployId, replicas, autoscaling, instances });
}
