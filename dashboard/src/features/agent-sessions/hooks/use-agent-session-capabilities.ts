import { useMemo } from "react";
import type { ApolloClient } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { AgentSessionCapabilitiesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { AgentSessionCapabilitiesView } from "@/features/agent-sessions/types";

// The hook's and the loader's one policy: what the loader warms is what the
// hook reads on mount (`PRIMED_FETCH_POLICY`'s rule, w9/m62).
const CAPABILITIES_FETCH_POLICY = "cache-first";

/**
 * Warms the read `useAgentSessionCapabilities(ownerId)` makes, with the same
 * document, variables and policy, from a route loader: the session header's
 * Terminal renders from it (w10/059), so a cold load that waited for the
 * client query inserted the button between Files and Connect after first
 * paint. Best-effort: it never rejects, and the hook re-runs a read that
 * failed here.
 */
export async function warmAgentSessionCapabilities(
  client: Pick<ApolloClient, "query">,
  ownerId: string,
): Promise<void> {
  await client
    .query({
      query: AgentSessionCapabilitiesDocument,
      variables: { ownerId },
      fetchPolicy: CAPABILITIES_FETCH_POLICY,
      errorPolicy: "all",
    })
    .catch(() => undefined);
}

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
      fetchPolicy: CAPABILITIES_FETCH_POLICY,
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
            sshEnabled: value.sshEnabled,
            terminalEnabled: value.terminalEnabled,
            agents: value.agents.map((agent) => agent.id),
            modelKeyReadyAgents: value.agents
              .filter((agent) => agent.modelKeyReady)
              .map((agent) => agent.id),
            models: Object.fromEntries(
              value.agents.map((agent) => [
                agent.id,
                { model: agent.model, models: agent.models },
              ]),
            ),
            sandbox: {
              defaultTemplate: value.defaultTemplate,
              templates: value.templates.map((template) => ({
                name: template.name,
                cpu: template.cpu,
                memoryGb: template.memoryGb,
              })),
            },
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
