import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { BlueprintDocument } from "@/graphql/definitions";
import { toBlueprintView } from "@/features/blueprints/lib/views";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import type { BlueprintView } from "@/features/blueprints/types";

export interface UseBlueprintResult {
  blueprint: BlueprintView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
}

export function useBlueprint(id: string): UseBlueprintResult {
  const { data, loading, error, refetch } = useQuery(BlueprintDocument, {
    // No ownerId: a Blueprint id resolves its own workspace server-side, so a
    // link to one in another of the caller's workspaces opens (w4/m169).
    variables: { id },
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  // The Render-compatible adapter may encode a missing row as an empty object
  // instead of GraphQL null. Treat an absent id as not-found so the detail page
  // never renders blank headings, epoch dates, and an actionable Sync button.
  const blueprint = useMemo(
    () => (data?.blueprint?.id ? toBlueprintView(data.blueprint) : null),
    [data],
  );

  return {
    blueprint,
    loading,
    error,
    refetch: () => {
      void refetch();
    },
  };
}
