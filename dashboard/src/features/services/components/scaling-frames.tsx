import type { ReactNode } from "react";
import { PlanBillingLink } from "@/common/components/plan-billing-link";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { CHART_FRAME_STYLE } from "@/features/metrics/components/chart-layout";
import { autoscalingLock } from "@/features/services/lib/autoscaling-lock";

// The Compute page's scaling cards' frames — headers, the plan notice and the
// loading cards — in a module light enough for the route skeletons' entry
// chunk, so the route's pending frame, the cards' own loading state and the
// ready cards draw the same geometry (w4/688, w6/m483).

/** The Autoscaling card's header; only the switch slot differs by state. */
export function AutoscalingHeader({ control }: { control: ReactNode }) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      {/* A phone puts the switch under the description: beside it, the
          switch squeezed the sentence into a column a third of the card
          wide and floated mid-paragraph. */}
      <div className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <CardTitle>{t("services.scalingTitle")}</CardTitle>
          <CardDescription className="mt-1">
            {t("services.scalingDescription")}
          </CardDescription>
        </div>
        <div className="flex items-center gap-2 shrink-0">{control}</div>
      </div>
    </CardHeader>
  );
}

/** Why the switch is held off; both plans are known before the read settles. */
export function AutoscalingLockedNotice({ planGated }: { planGated: boolean }) {
  const { t } = useTranslations();
  return (
    <p className="text-sm text-muted-foreground">
      {planGated ? (
        <>
          {t("services.scalingProPlanOnly")} <PlanBillingLink />
        </>
      ) : (
        t("services.scalingPaidOnly")
      )}
    </p>
  );
}

/**
 * The Autoscaling card while its read is pending: the autoscaling-off frame —
 * the common ready state, and the only one a free plan can show besides a
 * stored-but-inactive config. The switch and its label become blocks of their
 * ready size; the plan notice is plan-derived, so it shows as is. A paid
 * service whose autoscaling turns out to be on expands into the form: that
 * swap depends on data and is the one residual shift.
 */
export function AutoscalingLoadingCard({
  plan,
  workspacePlan,
}: {
  plan: string | null | undefined;
  workspacePlan: string | null | undefined;
}) {
  const { t } = useTranslations();
  const { locked, planGated } = autoscalingLock(plan, workspacePlan);
  return (
    <Card data-testid="autoscaling-loading">
      <AutoscalingHeader
        control={
          <>
            <Skeleton className="h-5 w-9 rounded-full" />
            <Skeleton className="rounded-sm">
              <span
                aria-hidden
                className="invisible block text-sm leading-none"
              >
                {t("services.scalingOff")}
              </span>
            </Skeleton>
          </>
        }
      />
      {locked && (
        <CardContent>
          <AutoscalingLockedNotice planGated={planGated} />
        </CardContent>
      )}
    </Card>
  );
}

/** The Manual Scaling card's header; a free plan's copy names its one instance. */
export function ManualScalingHeader({ isFree }: { isFree: boolean }) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("services.scalingManualTitle")}</CardTitle>
      <CardDescription className="mt-1">
        {isFree
          ? t("services.scalingManualDescriptionFree")
          : t("services.scalingManualDescription")}
      </CardDescription>
    </CardHeader>
  );
}

/**
 * The Manual Scaling card's frame while the autoscaling read resolves
 * (w4/688). The off state — this card showing — is the common outcome and the
 * only one on a free plan, so the slot is reserved at this card's geometry:
 * the same header and Instances label, the field row (the capped numeric input
 * alone on free, the slider row otherwise; both 32 px) and the Save button.
 */
export function ManualScalingSkeleton({ plan }: { plan?: string | null }) {
  const { t } = useTranslations();
  const isFree = plan === "free";
  return (
    <Card data-testid="manual-scaling-loading">
      <ManualScalingHeader isFree={isFree} />
      <CardContent>
        <div className="max-w-xl space-y-3">
          <p className="text-sm font-medium">
            {t("services.scalingManualInstances")}
          </p>
          {/* The ready row's input and Save are 44px tall below `lg`. */}
          <Skeleton
            className={cn(
              isFree ? "h-8 w-16" : "h-8 w-full",
              "max-lg:h-11 pointer-coarse:h-11",
            )}
          />
          <Skeleton className="h-8 w-28 max-lg:h-11 pointer-coarse:h-11" />
        </div>
      </CardContent>
    </Card>
  );
}

/** The Recent Metrics card's header; `viewAll` is its link to the Metrics tab. */
export function ScalingRecentMetricsHeader({
  viewAll,
}: {
  viewAll: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("services.scalingMetricsTitle")}</CardTitle>
      <CardDescription className="mt-1">
        {t("services.scalingMetricsNote")} {viewAll}
      </CardDescription>
    </CardHeader>
  );
}

/**
 * The Recent Metrics card while its reads are in flight: its header, then the
 * three sections' title rows (MetricSection's own classes) over the
 * chart-ratio shimmer each MetricSection draws before its first series.
 */
export function ScalingRecentMetricsSkeleton() {
  const { t } = useTranslations();
  const sections = [
    ["services.scalingMetricsMemory", true],
    ["services.scalingMetricsCPU", true],
    ["metrics.totalInstances", false],
  ] as const;
  return (
    <Card>
      <ScalingRecentMetricsHeader
        viewAll={
          <span className="underline underline-offset-2">
            {t("services.scalingMetricsViewAll")}
          </span>
        }
      />
      <CardContent className="space-y-6">
        {sections.map(([title, across]) => (
          <div key={title}>
            <div className="mb-2 flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
              <div className="text-sm font-medium text-foreground [overflow-wrap:normal]">
                {t(title)}
              </div>
              {across ? (
                <span className="text-xs text-muted-foreground">
                  {t("services.scalingMetricsAcross")}
                </span>
              ) : null}
            </div>
            <Skeleton className="w-full rounded-md" style={CHART_FRAME_STYLE} />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
