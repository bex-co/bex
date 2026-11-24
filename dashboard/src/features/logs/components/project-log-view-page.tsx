import { useCallback } from "react";
import { ProjectLogViewPageSkeleton } from "@/common/components/route-skeletons";
import { DEFAULT_LOG_RANGE } from "../lib/log-range";
import {
  projectLogContextHref,
  projectLogRange,
  type ProjectLogSearch,
} from "../lib/project-log-search";
import type { LogLine } from "../types";
import type { ProjectLogViewType } from "../lib/project-log-views";
import { viewLogType } from "../lib/view-filters";
import { type EventSourceFactory } from "../hooks/use-live-logs";
import { ProjectLogViewer } from "./project-log-viewer";
import { ProjectViewFrame, ProjectViewPage } from "./project-view-page";
import {
  LogsDeniedState,
  LogsGate,
} from "@/features/capabilities/components/logs-gate";

/**
 * A project's Application Logs or Request Logs view (w3/m505): Render's
 * `/project/{prj}/observability/logs/{application|request}?savedSearchId=…&r=…`
 * page, the log explorer over every resource the view reads, on Last hour
 * when neither the URL nor the view names a range, as Render's (w3/m537).
 *
 * Behind the role's logs gate (w9/145): a Viewer or a Billing member gets the
 * page's title and the role's reason, and neither the view's logs nor its
 * live tail are read.
 */
export function ProjectLogViewPage({
  projectId,
  viewType,
  search,
  onSearchChange,
  createEventSource,
}: {
  projectId: string;
  viewType: ProjectLogViewType;
  search: ProjectLogSearch;
  onSearchChange: (search: ProjectLogSearch) => void;
  createEventSource?: EventSourceFactory;
}) {
  // View in context opens this view around a line (w3/m528). Stable, so the
  // list's memoized rows keep their menu items across renders.
  const { savedSearchId } = search;
  const contextHref = useCallback(
    (line: LogLine) =>
      projectLogContextHref(projectId, viewType, savedSearchId, line),
    [projectId, viewType, savedSearchId],
  );
  return (
    <LogsGate
      fallback={
        <ProjectViewFrame viewType={viewType}>
          <LogsDeniedState />
        </ProjectViewFrame>
      }
    >
      <ProjectViewPage
        projectId={projectId}
        viewType={viewType}
        search={search}
        onSearchChange={onSearchChange}
        skeleton={<ProjectLogViewPageSkeleton />}
        iconName="ScrollText"
      >
        {({ viewId, scope, viewRange, onRangeChange }) => (
          <ProjectLogViewer
            key={viewId}
            scope={scope}
            logType={viewLogType(viewType)}
            range={projectLogRange(search, viewRange ?? DEFAULT_LOG_RANGE)}
            onRangeChange={onRangeChange}
            contextLine={search.contextLine}
            contextHref={contextHref}
            createEventSource={createEventSource}
          />
        )}
      </ProjectViewPage>
    </LogsGate>
  );
}
