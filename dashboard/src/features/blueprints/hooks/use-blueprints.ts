import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { BlueprintsDocument } from "@/graphql/definitions";
import type { BlueprintView } from "@/features/blueprints/types";
import { toBlueprintView } from "@/features/blueprints/lib/views";
import { nonNull } from "@/common/lib/non-null";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
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
    // Read the route loader's warm cache on mount (w9/m68); freshness comes from
    // the poll below and the loader's network-only fetch on every entry.
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const blueprints = useMemo(
    () => (data?.blueprints ?? []).filter(nonNull).map(toBlueprintView),
    [data],
  );

  return { blueprints, loading: !resolved || loading, error, refetch };
}
