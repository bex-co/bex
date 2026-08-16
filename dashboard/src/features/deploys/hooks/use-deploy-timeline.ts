import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
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
  skip = false,
): UseDeployTimelineResult {
  const { data, loading, error } = useQuery(DeployTimelineEventsDocument, {
    variables: {
      serviceId,
      startTime,
      endTime,
      limit: TIMELINE_LIMIT,
    },
    // Skip until the deploy's window is known — the page mounts this in parallel
    // with the header query (w9/m62 t002); a windowless fire would query the
    // wrong range.
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: endTime ? 0 : POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
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
