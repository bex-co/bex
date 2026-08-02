import { useQuery } from "@apollo/client/react";
import { BlueprintSyncsDocument } from "@/features/blueprints/api/operations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintSyncView } from "@/features/blueprints/types";

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

  const syncs: BlueprintSyncView[] = (data?.blueprintSyncs ?? []).filter(
    (s): s is BlueprintSyncView => s != null,
  );

  return {
    syncs,
    loading,
    error,
    refetch: () => {
      void refetch();
    },
  };
}
