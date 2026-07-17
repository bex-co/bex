import { useQuery } from "@apollo/client/react";
import { DeploysDocument } from "@/graphql/definitions";

export interface LatestDeploySummary {
  id: string;
  status: string;
}

export interface UseLatestDeployResult {
  deploy: LatestDeploySummary | null;
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads only the newest deploy for service-level chrome. The dedicated history
 * hook deliberately fetches a full page; the header needs just enough context
 * to name operator/App phase and deploy status as separate facts.
 */
export function useLatestDeploy(serviceId: string): UseLatestDeployResult {
  const { data, loading, error } = useQuery(DeploysDocument, {
    variables: { serviceId, limit: 1 },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const first = data?.deploys?.find((deploy) => !!deploy?.id);

  return {
    deploy: first ? { id: first.id ?? "", status: first.status ?? "" } : null,
    loading,
    error,
  };
}
