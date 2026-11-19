import { useQuery } from "@apollo/client/react";
import {
  SavedSearchesDocument,
  type SavedSearchFieldsFragment,
} from "@/graphql/definitions";

export type SavedSearch = SavedSearchFieldsFragment;

/**
 * A project's observability views (w3/m505): Render's saved searches, each a
 * named, typed filter document the whole project shares. The sidebar and the
 * view page read the same list (one query, one cache entry); the page's route
 * loader warms it.
 */
export function useSavedSearches(projectId: string): {
  views: SavedSearch[];
  loading: boolean;
  error: Error | undefined;
} {
  const { data, loading, error } = useQuery(SavedSearchesDocument, {
    variables: { projectId },
    errorPolicy: "all",
  });
  return {
    views: data?.savedSearches ?? [],
    loading: loading && !data,
    error,
  };
}
