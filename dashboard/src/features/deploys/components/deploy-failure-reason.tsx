import { cn } from "@/common/lib/utils/utils.ts";

/**
 * One cause presentation for every surface that shows why a deploy ended
 * (w1/m138 + w4/089): the deploy detail header, the deploys list row, and the
 * events feed row. `failureReason` is destructive (build/rollout failures);
 * `cancelReason` is neutral (superseded by a newer release). The treatment is
 * owned here; callers add layout only.
 */
export function DeployFailureReason({
  reason,
  className,
  truncate = false,
  tone = "destructive",
}: {
  reason: string | null | undefined;
  className?: string;
  /** Clamp to one line with a hover tooltip (list rows); default wraps. */
  truncate?: boolean;
  /** `destructive` for failures; `neutral` for supersede cancels (w4/089). */
  tone?: "destructive" | "neutral";
}) {
  if (!reason) return null;
  return (
    <p
      className={cn(
        "text-xs",
        tone === "destructive" ? "text-destructive" : "text-muted-foreground",
        truncate && "truncate",
        className,
      )}
      title={truncate ? reason : undefined}
    >
      {reason}
    </p>
  );
}
