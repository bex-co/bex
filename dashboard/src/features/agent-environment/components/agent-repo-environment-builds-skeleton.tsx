import type { ReactNode } from "react";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The build log view's frame, shared by the ready view and its skeleton so
 * the two cannot drift (dashboard/AGENTS.md skeleton rule): the build history
 * beside the selected build's log at md and up, stacked above it below md,
 * each region at a fixed height.
 */
export function AgentRepoEnvironmentBuildsFrame({
  list,
  panel,
}: {
  list: ReactNode;
  panel: ReactNode;
}) {
  return (
    <div
      className="grid gap-3 md:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]"
      data-testid="agent-repo-environment-builds"
    >
      <div
        className="h-40 overflow-y-auto rounded-md border md:h-96"
        data-skeleton-region="build-list"
      >
        {list}
      </div>
      <div
        className="flex h-80 min-w-0 flex-col overflow-hidden rounded-md border md:h-96"
        data-skeleton-region="build-panel"
      >
        {panel}
      </div>
    </div>
  );
}

/** Height of one history row, shared with the ready list. */
export const BUILD_ROW_CLASS = "flex h-14 w-full flex-col justify-center";

/** The log panel's header: the build's status line and its details line. */
export function AgentRepoEnvironmentBuildPanelHeader({
  children,
}: {
  children: ReactNode;
}) {
  return (
    <div
      className="flex h-14 shrink-0 flex-col justify-center gap-1 border-b px-3"
      data-testid="build-panel-header"
    >
      {children}
    </div>
  );
}

/** The build log view while the repository's history loads. */
export function AgentRepoEnvironmentBuildsSkeleton() {
  return (
    <div data-testid="agent-repo-environment-builds-skeleton">
      <AgentRepoEnvironmentBuildsFrame
        list={[0, 1, 2].map((i) => (
          <div key={i} className={`${BUILD_ROW_CLASS} gap-1.5 border-b px-3`}>
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        ))}
        panel={
          <>
            <AgentRepoEnvironmentBuildPanelHeader>
              <Skeleton className="h-5 w-1/3" />
              <Skeleton className="h-3 w-2/3" />
            </AgentRepoEnvironmentBuildPanelHeader>
            <div className="flex-1 space-y-2 p-3">
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-16 w-full" />
            </div>
          </>
        }
      />
    </div>
  );
}
