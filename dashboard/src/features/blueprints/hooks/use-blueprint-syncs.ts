import { useQuery } from "@apollo/client/react";
import { BlueprintSyncsDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintSyncView } from "@/features/blueprints/types";
import { toBlueprintSyncView } from "@/features/blueprints/lib/views";
import { nonNull } from "@/common/lib/non-null";

export interface UseBlueprintSyncsResult {
  syncs: BlueprintSyncView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
}

export function useBlueprintSyncs(
  blueprintId: string,
  limit = 20,
): UseBlueprintSyncsResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(BlueprintSyncsDocument, {
    variables: { id: blueprintId, ownerId: currentWorkspaceId, limit },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const syncs: BlueprintSyncView[] = (data?.blueprintSyncs ?? [])
    .filter(nonNull)
    .map(toBlueprintSyncView);

  return {
    syncs,
    loading,
    error,
    refetch: () => {
      void refetch();
    },
  };
}
