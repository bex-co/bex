import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RangeBound } from "@/features/metrics/lib/range";
import { workspacePlanNameKey } from "@/features/workspaces/types";
import { LogRetentionDocument } from "@/graphql/definitions";

const DAY_SECONDS = 24 * 60 * 60;

/**
 * The range bound a resource's log viewer applies (w9/m389): how far back its
 * workspace plan keeps logs, read from bex-api's `logRetention` — the same
 * floor every log read is clamped to — never from a client-side plan table.
 * The hint names the plan's window and, below the longest, the next plan's.
 *
 * The answer holds still between reads (the window's own floor slides on the
 * client's clock, `boundFloorMs`), so it is read once and again when the tab
 * regains focus, which is how an upgrade made elsewhere reaches an open page.
 * Undefined until it arrives, on an error, and where no plan applies (local
 * dev, no store): the picker is then unbounded, as the server is.
 */
export function useLogRetentionBound(resource: string): RangeBound | undefined {
  const { t } = useTranslations();
  const { data, refetch } = useQuery(LogRetentionDocument, {
    variables: { resource },
    errorPolicy: "all",
  });
  useEffect(() => {
    const onFocus = () => {
      if (document.visibilityState === "visible") void refetch();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
    };
  }, [refetch]);

  const retention = data?.logRetention;
  return useMemo(() => {
    if (!retention) return undefined;
    const planName = (plan: string) => {
      const key = workspacePlanNameKey(plan);
      return key ? t(key) : plan;
    };
    const kept = t("logs.retentionWindow", {
      count: retention.days,
      plan: planName(retention.plan),
    });
    const hint =
      retention.upgradePlan && retention.upgradeDays
        ? `${kept} ${t("logs.retentionUpgrade", {
            count: retention.upgradeDays,
            plan: planName(retention.upgradePlan),
          })}`
        : kept;
    return {
      maxSpanSeconds: retention.days * DAY_SECONDS,
      notBeforeMs: retention.visibleFrom
        ? Date.parse(retention.visibleFrom)
        : undefined,
      hint,
    };
  }, [retention, t]);
}
