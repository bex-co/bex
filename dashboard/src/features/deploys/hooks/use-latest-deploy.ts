import { useQuery } from "@apollo/client/react";
import { DeploysDocument } from "@/graphql/definitions";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import { isTerminalDeployStatus } from "@/features/deploys/lib/deploy-status";

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
 *
 * Polls at the converging cadence until that deploy reaches a terminal status
 * (w6/m46 t005). This is the header's deploy badge, and a deploy that closes
 * server-side fires no mutation in the browser — nothing else refetches this
 * document, so the poll interval IS how long the badge can sit on a status the
 * deploy has already left. A settled deploy falls back to the baseline: it will
 * never change again, and only a new deploy (a fresh row) can move this read.
 */
export function useLatestDeploy(serviceId: string): UseLatestDeployResult {
  const { data, loading, error, startPolling, stopPolling } = useQuery(
    DeploysDocument,
    {
      variables: { serviceId, limit: 1 },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const first = data?.deploys?.find((deploy) => !!deploy?.id);

  // Not yet loaded counts as converging, matching useServer/useDatabase.
  useConvergingPoll(
    startPolling,
    stopPolling,
    !first || !isTerminalDeployStatus(first.status ?? ""),
  );

  return {
    deploy: first ? { id: first.id ?? "", status: first.status ?? "" } : null,
    loading,
    error,
  };
}
