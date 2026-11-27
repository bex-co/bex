import { useMemo } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

/** A plan's figures as Render's review lists them: CPU, RAM, and the
 *  connection limit where the family has one (Key Value). */
export function useComputePlanFigures() {
  const { t, i18n } = useTranslations();
  const number = useMemo(
    () => new Intl.NumberFormat(i18n.language),
    [i18n.language],
  );
  return (plan: { cpu: string; memory: string; connectionLimit?: number }) => {
    const figures = [
      formatInstanceCPU(plan.cpu),
      `${formatInstanceMemory(plan.memory)} RAM`,
    ];
    if (plan.connectionLimit) {
      figures.push(
        t("common.planConnections", {
          count: plan.connectionLimit,
          formatted: number.format(plan.connectionLimit),
        }),
      );
    }
    return figures;
  };
}
