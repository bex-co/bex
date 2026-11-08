import type { ReactNode } from "react";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * The Version history tab's frame, shared by the ready tab and its skeleton so
 * the two cannot drift (dashboard/AGENTS.md skeleton rule): the version list
 * beside the version panel at md and up, stacked below it, each region at a
 * fixed height.
 */
export function AgentSkillHistoryFrame({
  list,
  panel,
}: {
  list: ReactNode;
  panel: ReactNode;
}) {
  return (
    <div
      className="grid gap-3 md:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]"
      data-testid="agent-skill-history"
    >
      <div
        className="h-44 overflow-y-auto rounded-md border md:h-96"
        data-skeleton-region="version-list"
      >
        {list}
      </div>
      <div
        className="flex h-72 min-w-0 flex-col overflow-hidden rounded-md border md:h-96"
        data-skeleton-region="version-panel"
      >
        {panel}
      </div>
    </div>
  );
}

/** Height of one version row, shared with the ready list. */
export const VERSION_ROW_CLASS = "flex h-16 w-full flex-col justify-center";

/** The panel's header: message line plus the view toggle and the load action. */
export function AgentSkillHistoryPanelHeader({
  meta,
  controls,
}: {
  meta: ReactNode;
  controls: ReactNode;
}) {
  return (
    <div className="shrink-0 space-y-2 border-b p-2">
      <div className="flex h-9 flex-col justify-center">{meta}</div>
      <div className="flex h-8 items-center justify-between gap-2">
        {controls}
      </div>
    </div>
  );
}

/** The Version history tab while its first page loads. */
export function AgentSkillHistorySkeleton() {
  return (
    <div data-testid="agent-skill-history-skeleton">
      <AgentSkillHistoryFrame
        list={[0, 1, 2].map((i) => (
          <div key={i} className={`${VERSION_ROW_CLASS} gap-1.5 border-b px-3`}>
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        ))}
        panel={
          <>
            <AgentSkillHistoryPanelHeader
              meta={
                <>
                  <Skeleton className="h-4 w-2/3" />
                  <Skeleton className="mt-1 h-3 w-1/2" />
                </>
              }
              controls={
                <>
                  <Skeleton className="h-8 w-40" />
                  <Skeleton className="h-8 w-32" />
                </>
              }
            />
            <div className="flex-1 space-y-2 p-3">
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-5/6" />
              <Skeleton className="h-3 w-2/3" />
            </div>
          </>
        }
      />
    </div>
  );
}
