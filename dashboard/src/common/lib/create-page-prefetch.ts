import type { DocumentNode } from "graphql";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import type { RouterContext } from "@/common/types/router-context";
import {
  GitConnectionsDocument,
  ProjectsDocument,
} from "@/graphql/definitions";

/**
 * A create page's loader: warms what its form reads on mount — the plan
 * catalog, the workspace's projects (the Project row) and, for a page with a
 * source picker, the Git connection — so a hover on the New menu or the type
 * switcher preloads them and the page opens complete instead of drawing its
 * plan and source skeletons (prefetch.ts). The repository list is left to the
 * page: bex-api refuses it to a viewer and records the denial, and the loader
 * cannot tell the role.
 */
export function prefetchCreatePage(
  {
    context,
    cause,
  }: { context: RouterContext; cause: "preload" | "enter" | "stay" },
  { catalog, git = false }: { catalog?: DocumentNode; git?: boolean },
): Promise<void> {
  const ownerId = context.workspaceId;
  const fetchPolicy = titleLoaderFetchPolicy(cause);
  const query =
    (document: DocumentNode, variables?: { ownerId: string }) => () =>
      context.client.query({
        query: document,
        variables,
        fetchPolicy,
        errorPolicy: "all",
      });
  return prefetchInParallel([
    ...(catalog ? [query(catalog)] : []),
    ...(ownerId == null
      ? []
      : [
          query(ProjectsDocument, { ownerId }),
          ...(git ? [query(GitConnectionsDocument, { ownerId })] : []),
        ]),
  ]);
}
