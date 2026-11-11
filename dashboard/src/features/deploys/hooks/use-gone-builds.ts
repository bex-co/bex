import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DeployBuildAvailabilityDocument } from "@/graphql/definitions";

/**
 * The ids of a service's recent deploys whose build the registry no longer
 * holds (w8/m156). The Events feed carries no per-deploy build state, so its
 * rows hide Rollback for these. A deploy that is not in the set, because it
 * is older than the newest 100 or the read has not answered, keeps its
 * action: the server refuses a missing build when the rollback is requested.
 */
export function useGoneBuilds(serviceId: string | null): ReadonlySet<string> {
  const { data } = useQuery(DeployBuildAvailabilityDocument, {
    variables: { serviceId: serviceId ?? "" },
    skip: !serviceId,
    fetchPolicy: "cache-and-network",
  });
  return useMemo(() => {
    const gone = new Set<string>();
    for (const deploy of data?.deploys ?? []) {
      if (deploy?.id && deploy.buildAvailable === false) gone.add(deploy.id);
    }
    return gone;
  }, [data]);
}
