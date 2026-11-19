import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { EmptyState } from "@/common/components/empty-state";
import { buttonVariants } from "@/common/components/ui/button";
import { PageFrame } from "@/common/components/page-frame";
import { ProjectLogViewPageSkeleton } from "@/common/components/route-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { SavedSearchType } from "@/graphql/definitions";
import { useSavedSearches } from "@/features/projects/hooks/use-saved-searches";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { DEFAULT_LOG_RANGE } from "../lib/log-search";
import {
  projectLogRange,
  projectLogRangeSearch,
  viewTimeRange,
  type ProjectLogSearch,
} from "../lib/project-log-search";
import { LOG_TYPE_APP, LOG_TYPE_REQUEST } from "../types";
import { type EventSourceFactory } from "../hooks/use-live-logs";
import {
  PROJECT_LOG_VIEW_TITLE_KEYS,
  type ProjectLogViewType,
} from "../lib/project-log-views";
import { ObservabilityViewActions } from "./observability-view-actions";
import { ObservabilityViewHeader } from "./observability-view-header";
import { ProjectLogViewer } from "./project-log-viewer";

/**
 * A project's Application Logs or Request Logs view (w3/m505): Render's
 * `/project/{prj}/observability/logs/{application|request}?savedSearchId=…&r=…`
 * page — the eyebrow and the view's name, then the log explorer over every
 * resource the view reads. The view is found in the project's saved searches
 * by its URL id; its `TIME_RANGE` filter is the range when the URL names none.
 * The header carries the view's Edit view, Duplicate and Delete (w3/m507).
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
  const { t } = useTranslations();
  const { views, loading, error } = useSavedSearches(projectId);
  // The view this page is deleting: once it leaves the list the page holds
  // its pending state until the next view opens, not "View not found".
  const [leaving, setLeaving] = useState<string | null>(null);
  const view = views.find(
    (v) => v.id === search.savedSearchId && v.type === viewType,
  );
  const fallbackView = views.find((v) => v.type === viewType);
  // The scope is the history query's and the tail's identity: it changes only
  // with the view's own filter document (the cache keeps that object), not
  // with another view's.
  const scope = useMemo(
    () => view && { project: projectId, filterDocument: view.filters },
    [view, projectId],
  );

  if (loading || leaving === search.savedSearchId) {
    return <ProjectLogViewPageSkeleton />;
  }
  if (!view || !scope) {
    // A failed read is not a missing view: say which it is.
    return (
      <PageFrame
        width="wide"
        header={
          <ObservabilityViewHeader
            name={t(PROJECT_LOG_VIEW_TITLE_KEYS[viewType])}
          />
        }
      >
        {error && views.length === 0 ? (
          <EmptyState
            role="alert"
            iconName="AlertCircle"
            title={t("logs.viewLoadErrorTitle")}
            description={t("logs.viewLoadErrorBody")}
          />
        ) : (
          <EmptyState
            iconName="ScrollText"
            title={t("logs.viewNotFoundTitle")}
            description={t("logs.viewNotFoundBody")}
            action={
              fallbackView ? (
                <ViewLink
                  projectId={projectId}
                  viewType={viewType}
                  savedSearchId={fallbackView.id}
                  label={t("logs.viewOpen", { name: fallbackView.name })}
                />
              ) : undefined
            }
          />
        )}
      </PageFrame>
    );
  }

  const range = projectLogRange(
    search,
    viewTimeRange(view.filters) ?? DEFAULT_LOG_RANGE,
  );
  const changeRange = (next: RangeSelection) =>
    onSearchChange({ ...search, ...projectLogRangeSearch(next) });
  return (
    <PageFrame
      width="wide"
      header={
        <ObservabilityViewHeader
          name={view.name}
          actions={
            <ObservabilityViewActions
              projectId={projectId}
              view={view}
              onLeaving={setLeaving}
            />
          }
        />
      }
    >
      <ProjectLogViewer
        key={view.id}
        scope={scope}
        logType={
          viewType === SavedSearchType.RequestLogs
            ? LOG_TYPE_REQUEST
            : LOG_TYPE_APP
        }
        range={range}
        onRangeChange={changeRange}
        createEventSource={createEventSource}
      />
    </PageFrame>
  );
}

function ViewLink({
  projectId,
  viewType,
  savedSearchId,
  label,
}: {
  projectId: string;
  viewType: ProjectLogViewType;
  savedSearchId: string;
  label: string;
}) {
  const className = buttonVariants({ variant: "outline", size: "sm" });
  return viewType === SavedSearchType.RequestLogs ? (
    <Link
      to="/project/$projectId/observability/logs/request"
      params={{ projectId }}
      search={{ savedSearchId }}
      className={className}
    >
      {label}
    </Link>
  ) : (
    <Link
      to="/project/$projectId/observability/logs/application"
      params={{ projectId }}
      search={{ savedSearchId }}
      className={className}
    >
      {label}
    </Link>
  );
}
