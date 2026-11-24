import { ProjectMetricsViewPageSkeleton } from "@/common/components/route-skeletons";
import { SavedSearchType } from "@/graphql/definitions";
import {
  ActivityDeniedState,
  ActivityGate,
} from "@/features/capabilities/components/activity-gate";
import {
  ProjectViewFrame,
  ProjectViewPage,
} from "@/features/logs/components/project-view-page";
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
 *
 * Behind the role's service-activity gate (w9/m621): a member who may not
 * view metrics gets the page's title and the role's reason, and neither the
 * view nor its charts are read.
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
    <ActivityGate
      fallback={
        <ProjectViewFrame viewType={SavedSearchType.Metrics}>
          <ActivityDeniedState />
        </ProjectViewFrame>
      }
    >
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
            range={projectMetricsRange(
              search,
              viewRange ?? DEFAULT_RANGE_PRESET,
            )}
            onRangeChange={onRangeChange}
          />
        )}
      </ProjectViewPage>
    </ActivityGate>
  );
}
