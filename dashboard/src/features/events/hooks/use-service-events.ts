import { useQuery } from "@apollo/client/react";
import {
  ServiceEventsDocument,
  type ServiceEventsQuery,
} from "@/graphql/definitions";

export type ServiceEventView = NonNullable<
  NonNullable<ServiceEventsQuery["serviceEvents"]>[number]
> & { id: string };

export interface UseServiceEventsResult {
  events: ServiceEventView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/** Shared service-event read used by both the Events feed and Metrics timeline. */
export function useServiceEvents(
  serviceId: string,
  limit: number,
): UseServiceEventsResult {
  const { data, loading, error, refetch } = useQuery(ServiceEventsDocument, {
    variables: { serviceId, limit },
    fetchPolicy: "cache-and-network",
  });
  const events = (data?.serviceEvents ?? []).filter(
    (event): event is ServiceEventView => event != null && !!event.id,
  );
  return { events, loading, error, refetch };
}
