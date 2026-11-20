import { useCallback } from "react";
import { ProjectLogViewPageSkeleton } from "@/common/components/route-skeletons";
import { DEFAULT_LOG_RANGE } from "../lib/log-search";
import {
  projectLogContextHref,
  type ProjectLogSearch,
} from "../lib/project-log-search";
import type { LogLine } from "../types";
import type { ProjectLogViewType } from "../lib/project-log-views";
import { viewLogType } from "../lib/view-filters";
import { type EventSourceFactory } from "../hooks/use-live-logs";
import { ProjectLogViewer } from "./project-log-viewer";
import { ProjectViewPage } from "./project-view-page";

/**
 * A project's Application Logs or Request Logs view (w3/m505): Render's
 * `/project/{prj}/observability/logs/{application|request}?savedSearchId=…&r=…`
 * page, the log explorer over every resource the view reads, at the Logs
 * tab's one-hour default when neither the URL nor the view names a range.
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
    <ProjectViewPage
      projectId={projectId}
      viewType={viewType}
      search={search}
      onSearchChange={onSearchChange}
      defaultRange={DEFAULT_LOG_RANGE}
      skeleton={<ProjectLogViewPageSkeleton />}
      iconName="ScrollText"
    >
      {({ viewId, scope, range, onRangeChange }) => (
        <ProjectLogViewer
          key={viewId}
          scope={scope}
          logType={viewLogType(viewType)}
          range={range}
          onRangeChange={onRangeChange}
          contextLine={search.contextLine}
          contextHref={contextHref}
          createEventSource={createEventSource}
        />
      )}
    </ProjectViewPage>
  );
}
