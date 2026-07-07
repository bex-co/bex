import { useMemo } from "react";
import { Lock, BarChart3, Heart } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AuthFeature } from "./index";

export function useAuthFeatures(): AuthFeature[] {
  const { t } = useTranslations();

  return useMemo(
    () => [
      {
        icon: Lock,
        title: t("auth.featureSecureTitle"),
        description: t("auth.featureSecureDescription"),
        iconColor: "text-blue-400 dark:text-blue-300",
        iconBg: "bg-blue-500/10 dark:bg-blue-400/10",
      },
      {
        icon: BarChart3,
        title: t("auth.featureDashboardTitle"),
        description: t("auth.featureDashboardDescription"),
        iconColor: "text-green-400 dark:text-green-300",
        iconBg: "bg-green-500/10 dark:bg-green-400/10",
      },
      {
        icon: Heart,
        title: t("auth.featureOpenSourceTitle"),
        description: t("auth.featureOpenSourceDescription"),
        iconColor: "text-purple-400 dark:text-purple-300",
        iconBg: "bg-purple-500/10 dark:bg-purple-400/10",
      },
    ],
    [t],
  );
}
