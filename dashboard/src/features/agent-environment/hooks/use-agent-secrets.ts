import { useCallback, useMemo } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentSecretsDocument,
  DeleteAgentSecretDocument,
  SetAgentSecretDocument,
} from "@/graphql/definitions";
import {
  toSecretScope,
  type AgentSecretScope,
  type AgentSecretView,
} from "@/features/agent-environment/lib/secrets";

/**
 * One create, rotate or rescope (ADR098 D9). Omit `value` to keep an existing
 * secret's value (a scope-only change); omit `scope` to keep its scope, or
 * for a new secret the server's default (workspace).
 */
export interface AgentSecretWrite {
  value?: string;
  scope?: AgentSecretScope;
  /** owner/name, only with the repo scope. */
  repo?: string;
}

export interface UseAgentSecretsResult {
  /** The secrets with their scopes, sorted by the server; never values (ADR098 D4). */
  secrets: AgentSecretView[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  set: (name: string, write: AgentSecretWrite) => Promise<void>;
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
    async (name: string, write: AgentSecretWrite) => {
      if (ownerId == null) throw new Error("no workspace selected");
      await setMutation({
        variables: {
          ownerId,
          name,
          value: write.value,
          scope: write.scope,
          repo: write.repo,
        },
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
  const secrets = useMemo<AgentSecretView[]>(
    () =>
      list?.secrets.map((secret) => ({
        name: secret.name,
        scope: toSecretScope(secret.scope),
        repo: secret.repo || null,
      })) ?? [],
    [list],
  );
  return {
    secrets,
    loading: ownerId == null || (loading && list == null),
    error,
    refetch,
    set,
    remove,
    saving: setting || deleting,
  };
}
