import type { RouterContext } from "@/common/types/router-context";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import type { en } from "@/i18n";
import { SavedSearchesDocument, SavedSearchType } from "@/graphql/definitions";

/** The two log view types a project's Observability group shows (w3/m505). */
export type ProjectLogViewType =
  SavedSearchType.ApplicationLogs | SavedSearchType.RequestLogs;

/** Every view type, in the order Render's docs list the default views: New
 *  View's TYPE menu (w3/m507; Service Metrics with its page, w3/m508). */
export const PROJECT_VIEW_TYPES: readonly SavedSearchType[] = [
  SavedSearchType.ApplicationLogs,
  SavedSearchType.RequestLogs,
  SavedSearchType.Metrics,
];

/** Each view type's own name: a new view's, and a page's without one. */
export const PROJECT_VIEW_TITLE_KEYS: Record<SavedSearchType, keyof typeof en> =
  {
    [SavedSearchType.ApplicationLogs]: "logs.applicationLogsTitle",
    [SavedSearchType.RequestLogs]: "logs.requestLogsTitle",
    [SavedSearchType.Metrics]: "metrics.serviceMetricsTitle",
  };

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
  type: SavedSearchType,
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
