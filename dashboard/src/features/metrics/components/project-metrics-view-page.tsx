import { ProjectMetricsViewPageSkeleton } from "@/common/components/route-skeletons";
import { SavedSearchType } from "@/graphql/definitions";
import { ProjectViewPage } from "@/features/logs/components/project-view-page";
import {
  projectMetricsRange,
  type ProjectLogSearch,
} from "@/features/logs/lib/project-log-search";
import { DEFAULT_RANGE_PRESET } from "@/features/metrics/lib/range";
import { ProjectMetricsView } from "./project-metrics-view";

/**
 * A project's Service Metrics view (w3/m508), at the address Render's bundle
 * routes it to: `/project/{prj}/observability/metrics?savedSearchId=…&r=…`,
 * at a service Metrics page's default (Last 12 hours) when neither the URL
 * nor the view names a range.
 */
export function ProjectMetricsViewPage({
  projectId,
  search,
  onSearchChange,
}: {
  projectId: string;
  search: ProjectLogSearch;
  onSearchChange: (search: ProjectLogSearch) => void;
}) {
  return (
    <ProjectViewPage
      projectId={projectId}
      viewType={SavedSearchType.Metrics}
      search={search}
      onSearchChange={onSearchChange}
      skeleton={<ProjectMetricsViewPageSkeleton />}
      iconName="ChartNoAxesCombined"
    >
      {({ viewId, scope, viewRange, onRangeChange }) => (
        <ProjectMetricsView
          key={viewId}
          scope={scope}
          range={projectMetricsRange(search, viewRange ?? DEFAULT_RANGE_PRESET)}
          onRangeChange={onRangeChange}
        />
      )}
    </ProjectViewPage>
  );
}
