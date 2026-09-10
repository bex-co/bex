import { cn } from "@/common/lib/utils/utils.ts";

/**
 * One failure-reason presentation for every surface that shows why a deploy
 * failed (w1/m138): the deploy detail header, the deploys list row, and the
 * events feed row. The backend sends `failureReason` only on failed deploys,
 * so its presence is the render gate — no status check needed. The treatment
 * (destructive, extra-small) is owned here; callers add layout only.
 */
export function DeployFailureReason({
  reason,
  className,
  truncate = false,
}: {
  reason: string | null | undefined;
  className?: string;
  /** Clamp to one line with a hover tooltip (list rows); default wraps. */
  truncate?: boolean;
}) {
  if (!reason) return null;
  return (
    <p
      className={cn("text-xs text-destructive", truncate && "truncate", className)}
      title={truncate ? reason : undefined}
    >
      {reason}
    </p>
  );
}
