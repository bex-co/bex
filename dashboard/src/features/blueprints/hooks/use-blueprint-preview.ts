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
  /**
   * The first review found resources the workspace already has, so the
   * Blueprint is to be created as a separate, suffixed set (w8/m153). It stays
   * true when the second review could not be loaded: `preview` is then null,
   * and a create must still not manage the existing resources.
   */
  matchesExisting: boolean;
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
 *
 * A Blueprint not yet created is reviewed in two steps when its file names
 * resources the workspace already has (w8/m153, Render's creation flow): the
 * first review reports the matches, and the second reviews the file as a
 * separate set of resources under a new name suffix. `preview` is then the
 * second review, whose existingResources is "create_new".
 */
export function useBlueprintPreview(
  repo: string,
  branch: string,
  path: string,
  /**
   * The blueprint this preview is on behalf of. The detail page's pre-sync
   * dialog passes its own id, so the server plans the sync against that
   * Blueprint's own resources and never reports them as another's;
   * `/blueprints/new` omits it, which is what makes the server report the
   * resources that already exist in the workspace (w8/m153).
   */
  forBlueprintId?: string,
  /**
   * The name the form would create the Blueprint under while the user has not
   * typed one. The review answers `suggestedName` when an approved Blueprint
   * already has it (w8/250). Omitted: the repository's name.
   */
  defaultName = "",
): UseBlueprintPreviewResult {
  const { currentWorkspaceId } = useWorkspace();

  const [debounced, setDebounced] = useState({
    repo,
    branch,
    path,
    defaultName,
  });
  useEffect(() => {
    const handle = setTimeout(
      () => setDebounced({ repo, branch, path, defaultName }),
      DEBOUNCE_MS,
    );
    return () => clearTimeout(handle);
  }, [repo, branch, path, defaultName]);

  // An empty branch previews the repository's default branch, which the
  // result reports back (w4/532); only the repo is required.
  const skip = !debounced.repo;
  const variables = {
    repo: debounced.repo,
    branch: debounced.branch || null,
    path: debounced.path || null,
    ownerId: currentWorkspaceId,
    blueprintId: forBlueprintId ?? null,
    // Only a form that proposes a name asks about it.
    ...(debounced.defaultName ? { name: debounced.defaultName } : {}),
  };
  const first = useQuery(BlueprintPreviewDocument, {
    variables,
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  // Decided by the first review alone. The second review lists the same
  // matches, so reading them from it would ask for a third.
  const reviewAsNew =
    !skip &&
    !forBlueprintId &&
    (first.data?.blueprintPreview?.matchedResources?.length ?? 0) > 0;
  const second = useQuery(BlueprintPreviewDocument, {
    variables: { ...variables, existingResources: "create_new" },
    skip: !reviewAsNew,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const settled =
    debounced.repo === repo &&
    debounced.branch === branch &&
    debounced.path === path &&
    debounced.defaultName === defaultName;

  const { data, error, refetch } = reviewAsNew ? second : first;
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
  // The same holds between the two reviews: once the first has matched, the
  // caller must not act before the second has answered.
  const awaitingSecond =
    reviewAsNew &&
    (second.loading || (second.data === undefined && !second.error));

  return {
    preview: skip ? null : preview,
    loading: awaitingDebounce || (!skip && (first.loading || awaitingSecond)),
    error,
    refetch,
    matchesExisting: reviewAsNew,
  };
}
