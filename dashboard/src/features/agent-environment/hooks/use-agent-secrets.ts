import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentSecretsDocument,
  DeleteAgentSecretDocument,
  SetAgentSecretDocument,
} from "@/graphql/definitions";

export interface UseAgentSecretsResult {
  /** Secret names, sorted by the server; never values (ADR098 D4). */
  names: string[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  set: (name: string, value: string) => Promise<void>;
  remove: (name: string) => Promise<void>;
  saving: boolean;
}

/** Lists and writes the workspace's agent secrets (ADR098). */
export function useAgentSecrets(ownerId: string | null): UseAgentSecretsResult {
  const { data, loading, error, refetch } = useQuery(AgentSecretsDocument, {
    variables: { ownerId: ownerId ?? "" },
    skip: ownerId == null,
    fetchPolicy: "cache-and-network",
  });
  const [setMutation, { loading: setting }] = useMutation(
    SetAgentSecretDocument,
  );
  const [deleteMutation, { loading: deleting }] = useMutation(
    DeleteAgentSecretDocument,
  );

  const set = useCallback(
    async (name: string, value: string) => {
      if (ownerId == null) throw new Error("no workspace selected");
      await setMutation({
        variables: { ownerId, name, value },
        refetchQueries: [AgentSecretsDocument],
        awaitRefetchQueries: true,
      });
    },
    [ownerId, setMutation],
  );

  const remove = useCallback(
    async (name: string) => {
      if (ownerId == null) throw new Error("no workspace selected");
      await deleteMutation({
        variables: { ownerId, name },
        refetchQueries: [AgentSecretsDocument],
        awaitRefetchQueries: true,
      });
    },
    [ownerId, deleteMutation],
  );

  const list = data?.agentSecrets ?? null;
  return {
    names: list?.secrets.map((secret) => secret.name) ?? [],
    loading: ownerId == null || (loading && list == null),
    error,
    refetch,
    set,
    remove,
    saving: setting || deleting,
  };
}
