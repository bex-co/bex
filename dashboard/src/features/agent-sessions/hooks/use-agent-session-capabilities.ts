import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionCapabilitiesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { AgentSessionCapabilitiesView } from "@/features/agent-sessions/types";

export interface UseAgentSessionCapabilitiesResult {
  capabilities: AgentSessionCapabilitiesView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * Reads the existing secret-free readiness projection. `ownerId` names the
 * workspace (an open session's own); omitted, the selected workspace.
 */
export function useAgentSessionCapabilities(
  ownerId?: string | null,
): UseAgentSessionCapabilitiesResult {
  const { currentWorkspaceId } = useWorkspace();
  const workspaceId = ownerId || currentWorkspaceId;
  const resolved = workspaceId != null;
  const { data, loading, error, refetch } = useQuery(
    AgentSessionCapabilitiesDocument,
    {
      variables: { ownerId: workspaceId },
      skip: !resolved,
      fetchPolicy: "cache-first",
      errorPolicy: "all",
    },
  );

  const value = data?.agentSessionCapabilities;
  // Memoized on the query result so consumers (attachment limits) get a
  // stable object between renders.
  const capabilities = useMemo<AgentSessionCapabilitiesView | null>(
    () =>
      value
        ? {
            enabled: value.enabled,
            modelKeyReady: value.modelKeyReady,
            agents: value.agents.map((agent) => agent.id),
            github: {
              connected: value.github.connected,
            },
            attachments: {
              enabled: value.attachments.enabled,
              maxBytes: value.attachments.maxBytes,
              maxPerTurn: value.attachments.maxPerTurn,
            },
          }
        : null,
    [value],
  );

  return {
    capabilities,
    loading: !resolved || loading,
    error,
    refetch,
  };
}
