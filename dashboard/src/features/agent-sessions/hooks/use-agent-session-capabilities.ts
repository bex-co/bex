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

/** Reads the existing secret-free readiness projection for the selected workspace. */
export function useAgentSessionCapabilities(): UseAgentSessionCapabilitiesResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(
    AgentSessionCapabilitiesDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      skip: !resolved,
      fetchPolicy: "cache-first",
      errorPolicy: "all",
    },
  );

  const value = data?.agentSessionCapabilities;
  const capabilities: AgentSessionCapabilitiesView | null = value
    ? {
        enabled: value.enabled,
        modelKeyReady: value.modelKeyReady,
        github: {
          connected: value.github.connected,
        },
      }
    : null;

  return {
    capabilities,
    loading: !resolved || loading,
    error,
    refetch,
  };
}
