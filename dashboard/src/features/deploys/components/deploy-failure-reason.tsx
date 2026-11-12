import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  deployReasonText,
  type DeployReasonTemplate,
} from "@/features/deploys/lib/deploy-reason";

/**
 * One cause presentation for every surface that shows why a deploy ended
 * (w1/m138 + w4/089): the deploy detail header, the deploys list row, and the
 * events feed row. `failureReason` is destructive (build/rollout failures);
 * `cancelReason` is neutral (superseded by a newer release). The treatment is
 * owned here; callers add layout only.
 */
export function DeployFailureReason({
  reason,
  template,
  className,
  truncate = false,
  tone = "destructive",
}: {
  reason: string | null | undefined;
  /** The reason's template, worded in the dashboard's language when known (w4/566). */
  template?: DeployReasonTemplate | null;
  className?: string;
  /** Clamp to one line with a hover tooltip (list rows); default wraps. */
  truncate?: boolean;
  /** `destructive` for failures; `neutral` for supersede cancels (w4/089). */
  tone?: "destructive" | "neutral";
}) {
  const { t } = useTranslations();
  if (!reason) return null;
  const text = deployReasonText(t, reason, template);
  return (
    <p
      className={cn(
        "text-xs",
        tone === "destructive" ? "text-destructive" : "text-muted-foreground",
        truncate && "truncate",
        className,
      )}
      title={truncate ? text : undefined}
    >
      {text}
    </p>
  );
}
