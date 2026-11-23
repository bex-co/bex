import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";

/**
 * The source picker's repository list: one height whether it is loading,
 * empty, failed or full, so the form under it never moves when the list
 * arrives. From `sm` it is the heading and five and a half rows: the half row
 * says the list goes on, where five whole rows read as a list of five.
 * `relative` makes it the rows' offset parent (the picker reads their
 * `offsetTop` from it).
 *
 * Kept apart from the picker, and free of hooks, so the create pages' pending
 * frames in route-skeletons.tsx draw the same box without pulling the picker
 * into their chunk.
 */
export const REPO_LIST_CLASS =
  "relative h-52 overflow-y-auto rounded-md border sm:h-[17rem]";
/** An account's heading, pinned while its rows scroll under it. */
export const REPO_HEADING_CLASS =
  "sticky top-0 z-10 flex items-center justify-between gap-3 border-b bg-muted px-3 py-1.5 text-xs font-medium text-muted-foreground";
// As many rows as the box holds.
const REPO_SKELETON_WIDTHS = ["w-40", "w-56", "w-32", "w-48", "w-36", "w-52"];

/** The list while its repositories load: the heading band over 44 px rows. */
export function RepoListSkeleton({ className }: { className?: string }) {
  return (
    <div
      inert
      aria-busy="true"
      data-repo-list
      className={cn(REPO_LIST_CLASS, "overflow-hidden", className)}
    >
      <div className={cn(REPO_HEADING_CLASS, "h-[29px]")} />
      {REPO_SKELETON_WIDTHS.map((width) => (
        <div key={width} className="flex h-11 items-center gap-2 px-3">
          <Skeleton className="size-4 shrink-0 rounded-full" />
          <Skeleton className={cn("h-4 max-w-full", width)} />
          <Skeleton className="ml-auto hidden h-3 w-10 sm:block" />
        </div>
      ))}
    </div>
  );
}
