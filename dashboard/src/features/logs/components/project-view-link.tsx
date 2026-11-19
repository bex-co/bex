import type { ComponentProps } from "react";
import { Link } from "@tanstack/react-router";
import { SavedSearchType } from "@/graphql/definitions";

/**
 * A link to a project view's page (w3/m505, Service Metrics w3/m508): its
 * type's route with the view as `savedSearchId`, as Render's sidebar links
 * name it. The sidebar's menu button hands its class and state attributes
 * through the rest of the props.
 */
export function ProjectViewLink({
  projectId,
  view,
  ...props
}: {
  projectId: string;
  view: { id: string; type: SavedSearchType };
} & Omit<ComponentProps<"a">, "href">) {
  const search = { savedSearchId: view.id };
  switch (view.type) {
    case SavedSearchType.RequestLogs:
      return (
        <Link
          {...props}
          to="/project/$projectId/observability/logs/request"
          params={{ projectId }}
          search={search}
        />
      );
    case SavedSearchType.Metrics:
      return (
        <Link
          {...props}
          to="/project/$projectId/observability/metrics"
          params={{ projectId }}
          search={search}
        />
      );
    default:
      return (
        <Link
          {...props}
          to="/project/$projectId/observability/logs/application"
          params={{ projectId }}
          search={search}
        />
      );
  }
}
