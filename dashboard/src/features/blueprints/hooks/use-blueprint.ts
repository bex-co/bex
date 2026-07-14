import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { BlueprintDocument } from "@/features/blueprints/api/operations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintView } from "@/features/blueprints/types";

export interface UseBlueprintResult {
  blueprint: BlueprintView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
}

export function useBlueprint(id: string): UseBlueprintResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(BlueprintDocument, {
    variables: { id, ownerId: currentWorkspaceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const blueprint = useMemo(() => data?.blueprint ?? null, [data]);

  return {
    blueprint,
    loading,
    error,
    refetch: () => { void refetch(); },
  };
}
