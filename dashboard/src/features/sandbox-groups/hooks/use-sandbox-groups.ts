import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  SandboxGroupDocument,
  SandboxGroupsDocument,
  type SandboxGroupQuery,
  type SandboxGroupsQuery,
} from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export type SandboxGroupItem = NonNullable<
  NonNullable<SandboxGroupsQuery["sandboxGroups"]>[number]
>;

export type SandboxGroupDetail = NonNullable<SandboxGroupQuery["sandboxGroup"]>;

export function useSandboxGroups() {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(SandboxGroupsDocument, {
    variables: { ownerId: currentWorkspaceId ?? undefined },
    skip: currentWorkspaceId == null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  // Memoized: the project page's environment cards derive their rows from it.
  const groups = useMemo(
    () =>
      (data?.sandboxGroups ?? []).filter(
        (g): g is SandboxGroupItem => g != null,
      ),
    [data],
  );

  return {
    groups,
    // Gate initial loading on having no data yet so background refetches never
    // unmount open dialogs or lose form drafts (dashboard/AGENTS.md).
    loading: (loading && data === undefined) || currentWorkspaceId == null,
    error,
    refetch,
  };
}

export function useSandboxGroup(id: string) {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(SandboxGroupDocument, {
    variables: { id, ownerId: currentWorkspaceId ?? undefined },
    skip: !id,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  return {
    group: data?.sandboxGroup ?? null,
    loading: loading && data === undefined,
    error,
    notFound: isNotFoundError(error),
    refetch,
  };
}
