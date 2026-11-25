import { useMemo, useState, type ReactNode } from "react";
import { EmptyState } from "@/common/components/empty-state";
import { Button } from "@/common/components/ui/button";
import { PageFrame } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import type { SavedSearchType } from "@/graphql/definitions";
import { useSavedSearches } from "@/features/projects/hooks/use-saved-searches";
import type { RangePreset } from "@/features/metrics/lib/range";
import type { LogRange } from "../lib/log-range";
import {
  projectLogRangeSearch,
  viewTimeRange,
  type ProjectLogSearch,
} from "../lib/project-log-search";
import { PROJECT_VIEW_TITLE_KEYS } from "../lib/project-log-views";
import { ObservabilityViewActions } from "./observability-view-actions";
import { ObservabilityViewHeader } from "./observability-view-header";
import { ProjectViewLink } from "./project-view-link";

/** What a view page's body reads: the view's scope, the time range the
 *  view saves (the range when the URL names none), and the range's writer. */
export interface ProjectViewBody {
  viewId: string;
  scope: { project: string; filterDocument: Record<string, unknown> };
  viewRange: RangePreset | null;
  onRangeChange: (range: LogRange) => void;
}

/**
 * A project observability view's page (w3/m505; Service Metrics, w3/m508):
 * Render's `/project/{prj}/observability/…?savedSearchId=…&r=…`. The view is
 * found in the project's saved searches by its URL id and type; its name
 * heads the page with its Edit view, Duplicate and Delete (w3/m507), and its
 * `timeRange` is the range when the URL names none. A missing view and
 * a failed read are told apart, and the page holds `skeleton` while the views
 * load and while the view it deletes leaves.
 */
export function ProjectViewPage({
  projectId,
  viewType,
  search,
  onSearchChange,
  skeleton,
  iconName,
  children,
}: {
  projectId: string;
  viewType: SavedSearchType;
  search: ProjectLogSearch;
  onSearchChange: (search: ProjectLogSearch) => void;
  skeleton: ReactNode;
  /** The not-found state's EmptyState icon. */
  iconName: string;
  children: (body: ProjectViewBody) => ReactNode;
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
  // The body's identity: it changes only with the view's own filter document
  // (the cache keeps that object), not with another view's.
  const scope = useMemo(
    () => view && { project: projectId, filterDocument: view.filters },
    [view, projectId],
  );

  if (loading || leaving === search.savedSearchId) {
    return skeleton;
  }
  if (!view || !scope) {
    return (
      <ProjectViewFrame viewType={viewType}>
        {error && views.length === 0 ? (
          <EmptyState
            role="alert"
            iconName="AlertCircle"
            title={t("logs.viewLoadErrorTitle")}
            description={t("logs.viewLoadErrorBody")}
          />
        ) : (
          <EmptyState
            iconName={iconName}
            title={t("logs.viewNotFoundTitle")}
            description={t("logs.viewNotFoundBody")}
            action={
              fallbackView ? (
                // A Button-slotted link, so the touch region sizes it.
                <Button asChild variant="outline" size="sm">
                  <ProjectViewLink projectId={projectId} view={fallbackView}>
                    {t("logs.viewOpen", { name: fallbackView.name })}
                  </ProjectViewLink>
                </Button>
              ) : undefined
            }
          />
        )}
      </ProjectViewFrame>
    );
  }

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
      {children({
        viewId: view.id,
        scope,
        viewRange: viewTimeRange(view.filters),
        onRangeChange: (next) =>
          onSearchChange({ ...search, ...projectLogRangeSearch(next) }),
      })}
    </PageFrame>
  );
}

/** A view page's frame with no view to name it: titled by its type. */
export function ProjectViewFrame({
  viewType,
  children,
}: {
  viewType: SavedSearchType;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <PageFrame
      width="wide"
      header={
        <ObservabilityViewHeader name={t(PROJECT_VIEW_TITLE_KEYS[viewType])} />
      }
    >
      {children}
    </PageFrame>
  );
}
