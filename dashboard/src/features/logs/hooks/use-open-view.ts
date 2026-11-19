import { useCallback } from "react";
import { useNavigate } from "@tanstack/react-router";
import { SavedSearchType } from "@/graphql/definitions";

/**
 * Opens a project view's page (w3/m507): a new, duplicated or remaining view
 * by its type's route and `savedSearchId`, Service Metrics too (w3/m508).
 * No view at all lands on the project's overview.
 */
export function useOpenView(projectId: string) {
  const navigate = useNavigate();
  return useCallback(
    (view: { id: string; type: SavedSearchType } | null | undefined) => {
      if (view?.type === SavedSearchType.RequestLogs) {
        return navigate({
          to: "/project/$projectId/observability/logs/request",
          params: { projectId },
          search: { savedSearchId: view.id },
        });
      }
      if (view?.type === SavedSearchType.Metrics) {
        return navigate({
          to: "/project/$projectId/observability/metrics",
          params: { projectId },
          search: { savedSearchId: view.id },
        });
      }
      if (view?.type === SavedSearchType.ApplicationLogs) {
        return navigate({
          to: "/project/$projectId/observability/logs/application",
          params: { projectId },
          search: { savedSearchId: view.id },
        });
      }
      return navigate({ to: "/project/$projectId", params: { projectId } });
    },
    [navigate, projectId],
  );
}
