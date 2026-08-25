import { cn } from "@/common/lib/utils/utils.ts";

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(
        "bg-accent animate-pulse rounded-md motion-reduce:animate-none group-data-[skeleton-frame=true]:!animate-none",
        className,
      )}
      {...props}
    />
  );
}

export { Skeleton };
