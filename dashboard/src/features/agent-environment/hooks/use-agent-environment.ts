import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentEnvironmentDocument,
  UpdateAgentEnvironmentDocument,
  type AgentEnvironmentInput,
} from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

/** The composer's readiness query (agent-sessions.graphql). */
const CAPABILITIES_QUERY = "AgentSessionCapabilities";

export interface UseAgentEnvironmentResult {
  environment: AgentEnvironmentView | null;
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  /** Sends a partial update; resolves the saved environment or throws. */
  update: (input: AgentEnvironmentInput) => Promise<AgentEnvironmentView>;
  saving: boolean;
}

/**
 * Reads and writes the workspace agent environment (w10/m17). The type is
 * keyed by `ownerId` in the Apollo cache, so a save refreshes the query.
 */
export function useAgentEnvironment(
  ownerId: string | null,
): UseAgentEnvironmentResult {
  const { data, loading, error, refetch } = useQuery(AgentEnvironmentDocument, {
    variables: { ownerId: ownerId ?? "" },
    skip: ownerId == null,
    fetchPolicy: "cache-and-network",
  });
  const [mutate, { loading: saving }] = useMutation(
    UpdateAgentEnvironmentDocument,
  );

  const update = useCallback(
    async (input: AgentEnvironmentInput) => {
      if (ownerId == null) throw new Error("no workspace selected");
      // Setting or clearing the model key changes the composer's
      // modelKeyReady, which the capabilities query carries.
      const keyChange =
        input.modelApiKey != null || input.clearModelApiKey === true;
      const result = await mutate({
        variables: { ownerId, input },
        refetchQueries: keyChange ? [CAPABILITIES_QUERY] : undefined,
      });
      const saved = result.data?.updateAgentEnvironment;
      if (!saved) throw new Error("updateAgentEnvironment returned no data");
      return saved;
    },
    [mutate, ownerId],
  );

  const environment = data?.agentEnvironment ?? null;
  return {
    environment,
    loading: ownerId == null || (loading && environment == null),
    error,
    refetch,
    update,
    saving,
  };
}
