import { Link } from "@tanstack/react-router";
import { CirclePause } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ServiceBase } from "@/features/services/lib/service-base";

const LINK_CLASS = "font-medium underline underline-offset-4";

/**
 * Render's Deploys-page banner for a service whose auto-deploys are off
 * (changelog 2026-09-03, w3/064): pushes to the branch no longer deploy, and
 * the setting that turns them back on is in Settings.
 */
export function AutoDeployOffBanner({
  serviceId,
  branch,
  base,
}: {
  serviceId: string;
  branch: string | null;
  base: ServiceBase;
}) {
  const { t } = useTranslations();
  return (
    <Alert data-testid="auto-deploy-off-banner">
      <CirclePause />
      <AlertTitle>{t("deploys.autoDeployOffTitle")}</AlertTitle>
      <AlertDescription>
        <p>
          {branch
            ? t("deploys.autoDeployOffBody", { branch })
            : t("deploys.autoDeployOffBodyNoBranch")}
        </p>
        <Link
          to={`${base}/$serviceId/settings`}
          params={{ serviceId }}
          className={LINK_CLASS}
        >
          {t("deploys.autoDeployOffSettings")}
        </Link>
      </AlertDescription>
    </Alert>
  );
}
