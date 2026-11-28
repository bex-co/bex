import { Check, TrendingDown, TrendingUp, X } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { isDowngrade } from "@/features/workspaces/lib/update-plan";
import {
  planComparisonRows,
  type PlanHighlight,
} from "@/features/workspaces/lib/plan-comparison";
import {
  workspacePlanEntry,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/**
 * Render's PlanComparisonCard (w3/m708), the confirmation's New plan summary:
 * the current plan beside the new one, each with its price and pitch, and the
 * highlights that differ, a trend arrow on the new side and a crossed-out line
 * for what the new plan does not have. bex's own numbers
 * (`lib/plan-comparison.ts`).
 */
export function PlanComparisonCard({
  current,
  next,
  pending = false,
}: {
  current: string;
  next: WorkspacePlanId;
  /** The pending frame's copy: the same card with its words reserved. */
  pending?: boolean;
}) {
  const { t } = useTranslations();
  const rows = planComparisonRows(current, next);
  const downgrade = isDowngrade(current, next);
  return (
    <div className="grid grid-cols-1 gap-3 @xl:grid-cols-2">
      <PlanSide
        pending={pending}
        plan={current}
        badge={t("workspaces.planComparisonCurrent")}
        highlights={rows.map((row) => row.current)}
        icon={<Check aria-hidden="true" className="size-4 shrink-0" />}
      />
      <PlanSide
        pending={pending}
        plan={next}
        badge={t("workspaces.planComparisonNew")}
        highlighted
        highlights={rows.map((row) => row.next)}
        icon={
          downgrade ? (
            <TrendingDown aria-hidden="true" className="size-4 shrink-0" />
          ) : (
            <TrendingUp aria-hidden="true" className="size-4 shrink-0" />
          )
        }
      />
    </div>
  );
}

function PlanSide({
  plan,
  badge,
  highlighted = false,
  highlights,
  icon,
  pending,
}: {
  plan: string;
  badge: string;
  highlighted?: boolean;
  highlights: PlanHighlight[];
  icon: React.ReactNode;
  pending: boolean;
}) {
  const { t } = useTranslations();
  // Pending, each line keeps its words' footprint under a bar.
  const words = (text: string) =>
    pending ? (
      <span className="relative">
        <ReservedText>{text}</ReservedText>
      </span>
    ) : (
      text
    );
  const entry = workspacePlanEntry(plan);
  return (
    <div
      className={cn(
        "flex flex-col gap-4 rounded-xl border p-5",
        highlighted ? "border-primary ring-2 ring-primary" : "border-border",
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-lg font-semibold">
          {words(entry ? t(entry.nameKey) : plan)}
        </span>
        <Badge variant={highlighted ? "default" : "secondary"}>{badge}</Badge>
      </div>
      {entry ? (
        <div className="space-y-2 text-sm">
          <p className="text-muted-foreground">
            {words(t(entry.descriptionKey))}
          </p>
          <p className="font-medium">{words(t(entry.billingKey))}</p>
        </div>
      ) : null}
      {highlights.length > 0 ? (
        <div className="space-y-2">
          <p className="text-muted-foreground font-mono text-xs tracking-wide uppercase">
            {words(t("workspaces.planComparisonHighlights"))}
          </p>
          <ul className="space-y-2 text-sm">
            {highlights.map((highlight) => (
              <li
                key={highlight.key}
                className={cn(
                  "flex items-start gap-2",
                  highlight.missing && "text-muted-foreground line-through",
                )}
              >
                <span className="mt-0.5">
                  {highlight.missing ? (
                    <X aria-hidden="true" className="size-4 shrink-0" />
                  ) : (
                    icon
                  )}
                </span>
                <span>{words(t(highlight.key, highlight.values))}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
