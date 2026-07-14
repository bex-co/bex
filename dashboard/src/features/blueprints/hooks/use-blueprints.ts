import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  BlueprintsDocument,
} from "@/features/blueprints/api/operations";
import type { BlueprintView } from "@/features/blueprints/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseBlueprintsResult {
  blueprints: BlueprintView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

export function useBlueprints(): UseBlueprintsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(BlueprintsDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const blueprints = useMemo(
    () =>
      (data?.blueprints ?? []).filter((b): b is BlueprintView => b != null),
    [data],
  );

  return { blueprints, loading: !resolved || loading, error, refetch };
}
