import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * An environment variable's or secret file's read row: key, value, then
 * Reveal and copy. A phone keeps the actions beside the key and puts the
 * value under it (two lines a row, not three with Reveal on its own line).
 * Shared with the rows' loading frames so the two stay one shape.
 */
export const ENV_ROW_CLASS =
  "grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-1 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-center sm:gap-3";
export const ENV_ROW_ACTIONS_CLASS =
  "flex flex-wrap justify-end gap-1 max-sm:col-start-2 max-sm:row-span-2 max-sm:row-start-1 max-sm:self-center";

/** Read rows while a list loads, at the controls' touch height below `lg`. */
export function EnvRowsSkeleton({ rows = 2 }: { rows?: number }) {
  return (
    <div className="divide-y" aria-hidden="true">
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className={ENV_ROW_CLASS}>
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-5 w-24" />
          <div className={ENV_ROW_ACTIONS_CLASS}>
            <Skeleton className="h-8 w-20 max-lg:h-11 pointer-coarse:h-11" />
            <Skeleton className="size-8 max-lg:size-11 pointer-coarse:size-11" />
          </div>
        </div>
      ))}
    </div>
  );
}
