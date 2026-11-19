import type { ReactNode } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { LegacyFreeNotice } from "@/features/services/components/instance-type-groups";

// The Compute section's frame and its current-plan row, in a module light
// enough for the route skeletons' entry chunk, so the Compute page's pending
// frame and its ready section draw the same geometry (w6/m483).

/**
 * The Compute section's card: its title, then the legacy-Free notice a
 * paid-only service's current plan implies, then the plan row or its editor.
 */
export function ComputePlanSectionFrame({
  serviceType,
  currentPlan,
  noticeId,
  children,
}: {
  serviceType: string | null;
  /** Undefined while unknown: no notice is drawn. */
  currentPlan: string | null | undefined;
  noticeId?: string;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.computeTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <LegacyFreeNotice
          id={noticeId}
          serviceType={serviceType}
          currentPlan={currentPlan}
        />
        {children}
      </CardContent>
    </Card>
  );
}

/** The current plan's summary and its Edit button in one bordered row. */
export function ComputePlanRowFrame({
  summary,
  action,
}: {
  summary: ReactNode;
  action: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">{summary}</div>
      <div className="shrink-0 self-start sm:self-auto">{action}</div>
    </div>
  );
}

/** The Compute section before the service is known: bars for the plan row. */
export function ComputePlanSectionSkeleton({
  serviceType,
  currentPlan,
}: {
  serviceType: string | null;
  currentPlan?: string | null;
}) {
  return (
    <ComputePlanSectionFrame
      serviceType={serviceType}
      currentPlan={currentPlan}
    >
      <ComputePlanRowFrame
        summary={<Skeleton className="h-5 w-56 max-w-full" />}
        action={<Skeleton className="h-8 w-[4.25rem]" />}
      />
    </ComputePlanSectionFrame>
  );
}
