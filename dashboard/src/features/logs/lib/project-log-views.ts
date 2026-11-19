import type { RouterContext } from "@/common/types/router-context";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import {
  SavedSearchesDocument,
  type SavedSearchType,
} from "@/graphql/definitions";

/** The two log view types a project's Observability group shows (w3/m505). */
export type ProjectLogViewType =
  SavedSearchType.ApplicationLogs | SavedSearchType.RequestLogs;

/**
 * The route loader's half of a view page: reads the project's views (warming
 * the cache the page and the sidebar read), and answers the id of the
 * project's first view of `type` — what a URL without `savedSearchId` opens,
 * as Render's sidebar links always name one. Undefined when the read fails or
 * the project has none; the page then says so.
 */
export async function defaultSavedSearchId(
  client: RouterContext["client"],
  projectId: string,
  type: ProjectLogViewType,
  cause: "preload" | "enter" | "stay",
): Promise<string | undefined> {
  const result = await client
    .query({
      query: SavedSearchesDocument,
      variables: { projectId },
      fetchPolicy: titleLoaderFetchPolicy(cause),
      errorPolicy: "all",
    })
    .catch(() => undefined);
  return result?.data?.savedSearches.find((view) => view.type === type)?.id;
}
