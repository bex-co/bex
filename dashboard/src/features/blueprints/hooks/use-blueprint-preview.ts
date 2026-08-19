import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@apollo/client/react";
import { BlueprintPreviewDocument } from "@/graphql/definitions";
import type { BlueprintPreviewResult } from "@/features/blueprints/types";
import { toBlueprintPreviewResult } from "@/features/blueprints/lib/views";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseBlueprintPreviewResult {
  preview: BlueprintPreviewResult | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/** Debounce typed inputs so path/branch keystrokes don't refire the fetch. */
const DEBOUNCE_MS = 400;

/**
 * Fetches the pre-create Blueprint review (Render's "Review Blueprint
 * configurations" step): bex-api pulls repo/branch/path from Git and dry-run
 * validates it. Skipped until both repo and branch are set. A missing file
 * comes back as preview.found === false with preview.error — only transport
 * failures surface through `error`.
 */
export function useBlueprintPreview(
  repo: string,
  branch: string,
  path: string,
): UseBlueprintPreviewResult {
  const { currentWorkspaceId } = useWorkspace();

  const [debounced, setDebounced] = useState({ repo, branch, path });
  useEffect(() => {
    const handle = setTimeout(
      () => setDebounced({ repo, branch, path }),
      DEBOUNCE_MS,
    );
    return () => clearTimeout(handle);
  }, [repo, branch, path]);

  const skip = !debounced.repo || !debounced.branch;
  const { data, loading, error, refetch } = useQuery(BlueprintPreviewDocument, {
    variables: {
      repo: debounced.repo,
      branch: debounced.branch,
      path: debounced.path || null,
      ownerId: currentWorkspaceId,
    },
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const settled =
    debounced.repo === repo &&
    debounced.branch === branch &&
    debounced.path === path;

  const preview = useMemo(
    () =>
      data?.blueprintPreview
        ? toBlueprintPreviewResult(data.blueprintPreview)
        : null,
    [data],
  );

  return {
    preview: skip ? null : preview,
    loading: !skip && (loading || !settled),
    error,
    refetch,
  };
}
