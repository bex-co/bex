import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentAutomationDocument,
  AgentAutomationRunsDocument,
  AgentAutomationsDocument,
  CreateAgentAutomationDocument,
  DeleteAgentAutomationDocument,
  PauseAgentAutomationDocument,
  ResumeAgentAutomationDocument,
  RunAgentAutomationDocument,
  TakeOverAgentAutomationDocument,
  UpdateAgentAutomationDocument,
  type AgentAutomationInput,
  type RunAgentAutomationMutation,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { AutomationView } from "@/features/agent-automations/hooks/use-agent-automations";

const LIST_REFETCH = [AgentAutomationsDocument];

export type AutomationRun = RunAgentAutomationMutation["runAgentAutomation"];

export interface UseAgentAutomationResult {
  automation: AutomationView | null;
  /** Loading with no data yet: the skeleton gate. */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/** One automation by aut- id or name. */
export function useAgentAutomation(id: string): UseAgentAutomationResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(AgentAutomationDocument, {
    variables: { ownerId: currentWorkspaceId ?? "", id },
    skip: currentWorkspaceId == null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const automation = data?.agentAutomation ?? null;
  return {
    automation,
    loading: currentWorkspaceId == null || (loading && automation == null),
    error,
    refetch,
  };
}

export interface UseAutomationMutations {
  create: (input: AgentAutomationInput) => Promise<AutomationView>;
  update: (id: string, input: AgentAutomationInput) => Promise<AutomationView>;
  remove: (id: string) => Promise<void>;
  pause: (id: string) => Promise<AutomationView>;
  resume: (id: string) => Promise<AutomationView>;
  takeOver: (id: string) => Promise<AutomationView>;
  runNow: (id: string) => Promise<AutomationRun>;
  /** Any mutation in flight, so a button can disable while one runs. */
  busy: boolean;
}

/**
 * Writes for automations. Each refetches the list so Mine/All, next-run and
 * last-run stay true; a run-now also refetches the run history.
 */
export function useAutomationMutations(): UseAutomationMutations {
  const { currentWorkspaceId } = useWorkspace();
  const ownerId = currentWorkspaceId ?? "";
  const [createM, c] = useMutation(CreateAgentAutomationDocument);
  const [updateM, u] = useMutation(UpdateAgentAutomationDocument);
  const [deleteM, d] = useMutation(DeleteAgentAutomationDocument);
  const [pauseM, p] = useMutation(PauseAgentAutomationDocument);
  const [resumeM, r] = useMutation(ResumeAgentAutomationDocument);
  const [takeOverM, t] = useMutation(TakeOverAgentAutomationDocument);
  const [runM, n] = useMutation(RunAgentAutomationDocument);

  const need = useCallback(() => {
    if (ownerId === "") throw new Error("no workspace selected");
    return ownerId;
  }, [ownerId]);

  const create = useCallback(
    async (input: AgentAutomationInput) => {
      const res = await createM({
        variables: { ownerId: need(), input },
        refetchQueries: LIST_REFETCH,
      });
      if (!res.data) throw new Error("createAgentAutomation returned nothing");
      return res.data.createAgentAutomation;
    },
    [createM, need],
  );
  const update = useCallback(
    async (id: string, input: AgentAutomationInput) => {
      const res = await updateM({
        variables: { ownerId: need(), id, input },
        refetchQueries: LIST_REFETCH,
      });
      if (!res.data) throw new Error("updateAgentAutomation returned nothing");
      return res.data.updateAgentAutomation;
    },
    [updateM, need],
  );
  const remove = useCallback(
    async (id: string) => {
      await deleteM({
        variables: { ownerId: need(), id },
        refetchQueries: LIST_REFETCH,
      });
    },
    [deleteM, need],
  );
  const pause = useCallback(
    async (id: string) => {
      const res = await pauseM({
        variables: { ownerId: need(), id },
        refetchQueries: LIST_REFETCH,
      });
      if (!res.data) throw new Error("pauseAgentAutomation returned nothing");
      return res.data.pauseAgentAutomation;
    },
    [pauseM, need],
  );
  const resume = useCallback(
    async (id: string) => {
      const res = await resumeM({
        variables: { ownerId: need(), id },
        refetchQueries: LIST_REFETCH,
      });
      if (!res.data) throw new Error("resumeAgentAutomation returned nothing");
      return res.data.resumeAgentAutomation;
    },
    [resumeM, need],
  );
  const takeOver = useCallback(
    async (id: string) => {
      const res = await takeOverM({
        variables: { ownerId: need(), id },
        refetchQueries: LIST_REFETCH,
      });
      if (!res.data)
        throw new Error("takeOverAgentAutomation returned nothing");
      return res.data.takeOverAgentAutomation;
    },
    [takeOverM, need],
  );
  const runNow = useCallback(
    async (id: string) => {
      const res = await runM({
        variables: { ownerId: need(), id },
        refetchQueries: [
          AgentAutomationsDocument,
          AgentAutomationDocument,
          AgentAutomationRunsDocument,
        ],
      });
      if (!res.data) throw new Error("runAgentAutomation returned nothing");
      return res.data.runAgentAutomation;
    },
    [runM, need],
  );

  return {
    create,
    update,
    remove,
    pause,
    resume,
    takeOver,
    runNow,
    busy:
      c.loading ||
      u.loading ||
      d.loading ||
      p.loading ||
      r.loading ||
      t.loading ||
      n.loading,
  };
}
