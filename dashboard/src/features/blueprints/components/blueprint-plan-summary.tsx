import type { ReactNode } from "react";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { EstimatedPricingPanel } from "./estimated-pricing-panel";
import type {
  BlueprintEstimatedPricing,
  BlueprintPreviewPlan,
} from "../types";

/** One named group of planned resources in a blueprint review. */
function PlanGroup({ label, names }: { label: string; names: string[] }) {
  if (names.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-sm text-muted-foreground">{label}:</span>
      {names.map((n) => (
        <Badge key={n} variant="secondary" className="text-xs">
          {n}
        </Badge>
      ))}
    </div>
  );
}

/**
 * The parsed-plan summary shared by the create page's review step and the
 * detail page's pre-sync dialog (w8/m21): resource groups + total count +
 * the estimated-pricing panel. Page-specific states (not-found, invalid,
 * prompts) stay with their pages.
 */
export function BlueprintPlanSummary({
  plan,
  pricing,
  note,
}: {
  plan: BlueprintPreviewPlan | null | undefined;
  pricing: BlueprintEstimatedPricing | null | undefined;
  note?: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <>
      <div className="space-y-3 rounded-md border p-4">
        <p className="text-sm font-medium">
          {t("blueprints.previewValid", {
            count: plan?.totalActions ?? 0,
          })}
        </p>
        <PlanGroup
          label={t("blueprints.previewServices")}
          names={(plan?.services ?? []).filter(Boolean)}
        />
        <PlanGroup
          label={t("blueprints.previewDatabases")}
          names={(plan?.databases ?? []).filter(Boolean)}
        />
        <PlanGroup
          label={t("blueprints.previewKeyValue")}
          names={(plan?.keyValue ?? []).filter(Boolean)}
        />
        <PlanGroup
          label={t("blueprints.previewEnvGroups")}
          names={(plan?.envGroups ?? []).filter(Boolean)}
        />
        {note}
      </div>
      <EstimatedPricingPanel pricing={pricing} />
    </>
  );
}
