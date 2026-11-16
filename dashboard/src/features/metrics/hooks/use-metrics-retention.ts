import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { useRefetchOnFocus } from "@/common/hooks/use-refetch-on-focus";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RangeBound } from "@/features/metrics/lib/range";
import { workspacePlanNameKey } from "@/features/workspaces/types";
import { MetricsRetentionDocument } from "@/graphql/definitions";

const DAY_SECONDS = 24 * 60 * 60;

/**
 * The range bound a resource's metrics charts apply (w7/m427): how far back
 * its workspace plan keeps metrics, read from bex-api's `metricsRetention` —
 * the same floor every chart read is clamped to — never from a client-side
 * plan table. The hint names the plan's window and every longer one with the
 * plan that has it, so each disabled preset is explained.
 *
 * The answer holds still between reads (the floor slides on the client's
 * clock, `boundFloorMs`), so it is read once and again when the tab regains
 * focus, which is how an upgrade made elsewhere reaches an open page.
 * Undefined until it arrives, on an error, and where no plan applies (local
 * dev, no store): the picker is then unbounded, as the server is.
 */
export function useMetricsRetentionBound(
  resource: string,
): RangeBound | undefined {
  const { t } = useTranslations();
  const { data, refetch } = useQuery(MetricsRetentionDocument, {
    variables: { resource },
    errorPolicy: "all",
  });
  useRefetchOnFocus(refetch);

  const retention = data?.metricsRetention;
  return useMemo(() => {
    if (!retention) return undefined;
    const planName = (plan: string) => {
      const key = workspacePlanNameKey(plan);
      return key ? t(key) : plan;
    };
    const hint = [
      t("metrics.retentionWindow", {
        count: retention.days,
        plan: planName(retention.plan),
      }),
      ...retention.upgrades.map((upgrade) =>
        t("metrics.retentionUpgrade", {
          count: upgrade.days,
          plan: planName(upgrade.plan),
        }),
      ),
    ].join(" ");
    return { maxSpanSeconds: retention.days * DAY_SECONDS, hint };
  }, [retention, t]);
}
