import { cn } from "@/common/lib/utils/utils.ts";

const skeletonClasses =
  "bg-accent animate-pulse rounded-md motion-reduce:animate-none group-data-[skeleton-frame=true]:!animate-none";

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(skeletonClasses, className)}
      {...props}
    />
  );
}

/**
 * A skeleton bar that is phrasing content, so it may sit inside a `<p>`,
 * `<label>` or `<span>`: a `<div>` there is moved out of the paragraph by the
 * HTML parser, so the server's markup no longer matches the client tree and
 * hydration fails (React #418, w4/686). Absolutely positioned, it lays out as a
 * block over its `relative` parent.
 */
function SkeletonSpan({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="skeleton"
      className={cn(skeletonClasses, "absolute inset-0", className)}
      {...props}
    />
  );
}

export { Skeleton, SkeletonSpan };
