import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { EnvironmentIpAllowListDocument } from "@/graphql/definitions";
import {
  toIPAllowListDrafts,
  type IPAllowListEntryDraft,
} from "@/common/lib/ip-allow-list";
import { useWorkspace } from "@/features/workspaces/context";
import { useWorkspaceIPAllowList } from "@/features/workspaces/hooks/use-workspace-ip-allow-list";

/** Where an environment's rules are edited: its settings' Networking card. */
function environmentIPRulesHref(projectId: string, environmentId: string) {
  return `/project/${projectId}/environment/${environmentId}/settings#networking`;
}

/**
 * The higher levels of inbound IP rules that also apply to a resource: the
 * current workspace's always, and its environment's when it has one.
 * `loading` until every level that applies is read; `error` when one could
 * not be.
 */
export type InboundIPLevelsRead =
  | { state: "loading" }
  | { state: "error" }
  | {
      state: "ready";
      workspace: IPAllowListEntryDraft[];
      environment?: { entries: IPAllowListEntryDraft[]; href: string };
    };

/**
 * Reads the levels Render's Networking view shows above a resource's own
 * rules (w4/m710): the owner's list (`workspace(id).ipAllowList`) and, given
 * an environment, `environment(id).ipAllowListEntries` with the project its
 * View rules link needs. Both `cache-and-network`, as Render's, so a panel
 * opened after a level changed elsewhere shows the change.
 */
export function useInboundIPLevels(
  environmentId: string | null | undefined,
): InboundIPLevelsRead {
  const { currentWorkspace } = useWorkspace();
  const workspace = useWorkspaceIPAllowList(currentWorkspace?.id, {
    fetchPolicy: "cache-and-network",
  });
  const environmentQuery = useQuery(EnvironmentIpAllowListDocument, {
    variables: { id: environmentId ?? "" },
    skip: !environmentId,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const environment = environmentQuery.data?.environment;
  const environmentEntries = useMemo(
    () => toIPAllowListDrafts(environment?.ipAllowListEntries),
    [environment],
  );

  if (
    !currentWorkspace ||
    workspace.loading ||
    (environmentId &&
      environmentQuery.loading &&
      environmentQuery.data === undefined)
  ) {
    return { state: "loading" };
  }
  if (!workspace.entries) return { state: "error" };
  if (!environmentId) return { state: "ready", workspace: workspace.entries };
  if (!environmentEntries || !environment?.projectId) return { state: "error" };
  return {
    state: "ready",
    workspace: workspace.entries,
    environment: {
      entries: environmentEntries,
      href: environmentIPRulesHref(environment.projectId, environmentId),
    },
  };
}
