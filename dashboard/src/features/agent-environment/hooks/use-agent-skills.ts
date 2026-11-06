import { useCallback } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import {
  AgentSkillDocument,
  AgentSkillsDocument,
  CreateAgentSkillDocument,
  DeleteAgentSkillDocument,
  UpdateAgentSkillDocument,
  type AgentSkillInput,
  type AgentSkillQuery,
  type AgentSkillsQuery,
} from "@/graphql/definitions";

/** A skill as the list carries it: no body (ADR099 D3). */
export type AgentSkillSummary =
  AgentSkillsQuery["agentSkills"]["skills"][number];
/** A skill read on its own: body plus the whole SKILL.md as content. */
export type AgentSkillDetail = AgentSkillQuery["agentSkill"];

export interface UseAgentSkillsResult {
  skills: AgentSkillSummary[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  /** One skill with its body and content, read fresh. */
  read: (skill: string) => Promise<AgentSkillDetail>;
  create: (input: AgentSkillInput) => Promise<void>;
  update: (skill: string, input: AgentSkillInput) => Promise<void>;
  remove: (skill: string) => Promise<void>;
  saving: boolean;
}

/** Lists, reads and writes the workspace's skills for cloud agents (ADR099). */
export function useAgentSkills(ownerId: string | null): UseAgentSkillsResult {
  const client = useApolloClient();
  const { data, loading, error, refetch } = useQuery(AgentSkillsDocument, {
    variables: { ownerId: ownerId ?? "" },
    skip: ownerId == null,
    fetchPolicy: "cache-and-network",
  });
  const [createMutation, { loading: creating }] = useMutation(
    CreateAgentSkillDocument,
  );
  const [updateMutation, { loading: updating }] = useMutation(
    UpdateAgentSkillDocument,
  );
  const [deleteMutation, { loading: deleting }] = useMutation(
    DeleteAgentSkillDocument,
  );

  const requireOwner = useCallback(() => {
    if (ownerId == null) throw new Error("no workspace selected");
    return ownerId;
  }, [ownerId]);

  const read = useCallback(
    async (skill: string) => {
      const res = await client.query({
        query: AgentSkillDocument,
        variables: { ownerId: requireOwner(), skill },
        fetchPolicy: "network-only",
      });
      if (!res.data) throw new Error("agentSkill returned no skill");
      return res.data.agentSkill;
    },
    [client, requireOwner],
  );

  const create = useCallback(
    async (input: AgentSkillInput) => {
      await createMutation({
        variables: { ownerId: requireOwner(), input },
        refetchQueries: [AgentSkillsDocument],
        awaitRefetchQueries: true,
      });
    },
    [createMutation, requireOwner],
  );

  const update = useCallback(
    async (skill: string, input: AgentSkillInput) => {
      await updateMutation({
        variables: { ownerId: requireOwner(), skill, input },
        refetchQueries: [AgentSkillsDocument],
        awaitRefetchQueries: true,
      });
    },
    [updateMutation, requireOwner],
  );

  const remove = useCallback(
    async (skill: string) => {
      await deleteMutation({
        variables: { ownerId: requireOwner(), skill },
        refetchQueries: [AgentSkillsDocument],
        awaitRefetchQueries: true,
      });
    },
    [deleteMutation, requireOwner],
  );

  const list = data?.agentSkills ?? null;
  return {
    skills: list?.skills ?? [],
    loading: ownerId == null || (loading && list == null),
    error,
    refetch,
    read,
    create,
    update,
    remove,
    saving: creating || updating || deleting,
  };
}
