import { useQuery } from "@apollo/client/react";
import { ServiceInstancesDocument } from "@/graphql/definitions";

export interface ServiceInstance {
  id: string;
  createdAt: string;
}

/**
 * Reads bex-api's `serviceInstances(id)` query — the running instances of a
 * service ({id, createdAt}), Render's per-service instance list. Backs the Web
 * Shell instance picker (w2/m55). Presentation only; the same ListInstances
 * verb REST exposes at GET /v1/services/{id}/instances.
 */
export function useServiceInstances(id: string) {
  const { data, loading, refetch } = useQuery(ServiceInstancesDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const instances: ServiceInstance[] = (data?.serviceInstances ?? [])
    .filter((i): i is NonNullable<typeof i> => Boolean(i?.id))
    .map((i) => ({ id: i.id ?? "", createdAt: i.createdAt ?? "" }));

  return { instances, loading, refetch };
}
