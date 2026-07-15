import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  DeployTimelineEventsDocument,
  type DeployTimelineEventsQuery,
} from "@/graphql/definitions";

const POLL_INTERVAL_MS = 3000;
const TIMELINE_LIMIT = 100;

type ServiceEventNode = NonNullable<
  NonNullable<DeployTimelineEventsQuery["serviceEvents"]>[number]
>;

export interface DeployTimelineEvent {
  id: string;
  type: string;
  timestamp: string | null;
  deployId: string;
  deployStatus: string;
}

export interface UseDeployTimelineResult {
  events: DeployTimelineEvent[];
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads the service-events feed inside one deploy's time window, then retains
 * only transition events whose explicit details.deployId matches this deploy.
 * ServiceEvent.id is an evt-… id and must never be treated as a deploy id.
 */
export function useDeployTimeline(
  serviceId: string,
  deployId: string,
  startTime: string | undefined,
  endTime: string | undefined,
): UseDeployTimelineResult {
  const { data, loading, error } = useQuery(DeployTimelineEventsDocument, {
    variables: {
      serviceId,
      startTime,
      endTime,
      limit: TIMELINE_LIMIT,
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: endTime ? 0 : POLL_INTERVAL_MS,
  });

  const events = useMemo(
    () =>
      (data?.serviceEvents ?? [])
        .filter(
          (event): event is ServiceEventNode =>
            !!event && event.details?.deployId === deployId,
        )
        .map((event) => ({
          id: event.id ?? "",
          type: event.type ?? "",
          timestamp: event.timestamp ?? null,
          deployId: event.details?.deployId ?? "",
          deployStatus: event.details?.deployStatus ?? "",
        })),
    [data, deployId],
  );

  return { events, loading: loading && !data, error };
}
