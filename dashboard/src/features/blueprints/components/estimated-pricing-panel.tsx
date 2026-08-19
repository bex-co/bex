import { useTranslations } from "@/common/hooks/use-translations";
import { Badge } from "@/common/components/ui/badge";
import type { BlueprintEstimatedPricing } from "../types";

/** Reason string → row badge + footnote phrase translation keys. Keyed openly
 * (see BlueprintVariableCost.reason): an unknown reason falls back to the
 * generic "Variable" badge below. */
const REASONS: Record<string, { labelKey: string; excludeKey: string }> = {
  autoscaling: {
    labelKey: "blueprints.pricingReasonAutoscaling",
    excludeKey: "blueprints.pricingExcludeAutoscaling",
  },
  multi_instance: {
    labelKey: "blueprints.pricingReasonMultiInstance",
    excludeKey: "blueprints.pricingExcludeMultiInstance",
  },
  cron: {
    labelKey: "blueprints.pricingReasonCron",
    excludeKey: "blueprints.pricingExcludeCron",
  },
};

/**
 * Render-style "Estimated pricing" panel for the Blueprint review step
 * (w8/m18): per-resource `(PlanName) $X / month` rows, database rows with the
 * instance/storage breakdown, runtime-dependent resources listed but excluded
 * from the total. Hidden entirely when there are no priced lines (all-free
 * blueprint), like Render's panel.
 */
export function EstimatedPricingPanel({
  pricing,
}: {
  pricing: BlueprintEstimatedPricing | null | undefined;
}) {
  const { t } = useTranslations();
  const lines = (pricing?.lines ?? []).filter(Boolean);
  const variable = (pricing?.variable ?? []).filter(Boolean);
  if (lines.length === 0) return null;

  const exclusionKeys = [
    ...new Set(
      variable
        .map((v) => REASONS[v.reason]?.excludeKey)
        .filter((k): k is string => !!k),
    ),
  ];
  const marker = exclusionKeys.length > 0 ? "*" : "";

  return (
    <div className="space-y-3 rounded-md border p-4">
      <div>
        <p className="text-sm font-medium">{t("blueprints.pricingTitle")}</p>
        <p className="text-sm text-muted-foreground">
          {t("blueprints.pricingSubtitle")}
        </p>
      </div>
      <ul className="divide-y">
        {lines.map((line, i) => (
          <li
            key={`${line.name}-${i}`}
            className="flex items-baseline justify-between gap-3 py-1.5"
          >
            <span className="min-w-0 truncate text-sm">{line.name}</span>
            <span className="shrink-0 text-right">
              <span className="text-sm tabular-nums">
                {t("blueprints.pricingLineAmount", {
                  tier: line.tierLabel,
                  amount: line.monthlyUsd,
                })}
              </span>
              {line.storageUsd && line.storageGb ? (
                <span className="block text-xs text-muted-foreground tabular-nums">
                  {t("blueprints.pricingLineBreakdown", {
                    instance: line.instanceUsd ?? "0.00",
                    gb: line.storageGb,
                    storage: line.storageUsd,
                  })}
                </span>
              ) : null}
            </span>
          </li>
        ))}
        {variable.map((v, i) => (
          <li
            key={`var-${v.name}-${i}`}
            className="flex items-baseline justify-between gap-3 py-1.5"
          >
            <span className="flex min-w-0 items-baseline gap-2">
              <span className="truncate text-sm">{v.name}</span>
              <Badge variant="outline" className="shrink-0 text-xs">
                {t(REASONS[v.reason]?.labelKey ?? "blueprints.pricingVariable")}
              </Badge>
            </span>
            <span className="shrink-0 text-sm text-muted-foreground">
              {t("blueprints.pricingVariable")}
            </span>
          </li>
        ))}
      </ul>
      <div className="flex items-baseline justify-between border-t pt-2">
        <span className="text-sm font-semibold">
          {t("blueprints.pricingTotalLabel")}
        </span>
        <span className="text-sm font-semibold tabular-nums">
          {t("blueprints.pricingTotalAmount", {
            amount: pricing?.totalUsd ?? "0.00",
            marker,
          })}
        </span>
      </div>
      {exclusionKeys.length > 0 ? (
        <p className="text-xs text-muted-foreground">
          {t("blueprints.pricingExclusions", {
            items: exclusionKeys.map((k) => t(k)).join(", "),
          })}
        </p>
      ) : null}
      <p className="text-xs text-muted-foreground">
        {t("blueprints.pricingEstimateNote")}
      </p>
    </div>
  );
}
