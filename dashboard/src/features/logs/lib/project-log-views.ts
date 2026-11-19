import type { RouterContext } from "@/common/types/router-context";
import { titleLoaderFetchPolicy } from "@/common/lib/document-head";
import type { en } from "@/i18n";
import { SavedSearchesDocument, SavedSearchType } from "@/graphql/definitions";

/** The two log view types a project's Observability group shows (w3/m505). */
export type ProjectLogViewType =
  SavedSearchType.ApplicationLogs | SavedSearchType.RequestLogs;

/** Those types in Render's order: the sidebar's list and New View's TYPE
 *  menu (w3/m507). Service Metrics joins with its page (w3/m508). */
export const PROJECT_LOG_VIEW_TYPES: readonly ProjectLogViewType[] = [
  SavedSearchType.ApplicationLogs,
  SavedSearchType.RequestLogs,
];

/** Each log view type's own name: a new view's, and a page's without one. */
export const PROJECT_LOG_VIEW_TITLE_KEYS: Record<
  ProjectLogViewType,
  keyof typeof en
> = {
  [SavedSearchType.ApplicationLogs]: "logs.applicationLogsTitle",
  [SavedSearchType.RequestLogs]: "logs.requestLogsTitle",
};

/** Whether a view has a log page (Service Metrics has none yet). */
export function isProjectLogView<T extends { type: SavedSearchType }>(
  view: T,
): view is T & { type: ProjectLogViewType } {
  return (PROJECT_LOG_VIEW_TYPES as readonly SavedSearchType[]).includes(
    view.type,
  );
}

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
