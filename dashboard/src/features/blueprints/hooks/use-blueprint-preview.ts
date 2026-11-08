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
 * validates it. Skipped until the repo is set; with no branch it reviews the
 * repository's default branch and reports which. A missing file
 * comes back as preview.found === false with preview.error — only transport
 * failures surface through `error`.
 */
export function useBlueprintPreview(
  repo: string,
  branch: string,
  path: string,
  /**
   * The blueprint this preview is on behalf of. The detail page's pre-sync
   * dialog passes its own id so the server does not report the blueprint as
   * conflicting with itself; `/blueprints/new` omits it, which is what makes
   * "a live blueprint already tracks this repo+branch" visible before Deploy
   * rather than at apply time (w4/m125).
   */
  forBlueprintId?: string,
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

  // An empty branch previews the repository's default branch, which the
  // result reports back (w4/532); only the repo is required.
  const skip = !debounced.repo;
  const { data, loading, error, refetch } = useQuery(BlueprintPreviewDocument, {
    variables: {
      repo: debounced.repo,
      branch: debounced.branch || null,
      path: debounced.path || null,
      ownerId: currentWorkspaceId,
      blueprintId: forBlueprintId ?? null,
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

  // Inputs that just became complete are pending from that very render: the
  // query is still skipped for one debounce, and a caller that read "not
  // loading" then would offer its action before anything was reviewed.
  const awaitingDebounce = !settled && (!skip || repo !== "");

  return {
    preview: skip ? null : preview,
    loading: awaitingDebounce || (!skip && loading),
    error,
    refetch,
  };
}
