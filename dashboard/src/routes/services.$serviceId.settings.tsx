import { createFileRoute } from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { InstanceTypeRow } from "@/features/services/components/instance-type-row";
import { IdleTimeoutRow } from "@/features/services/components/idle-timeout-row";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";
import { CustomDomainsSection } from "@/features/services/components/custom-domains-section";
import { PlatformSubdomainSection } from "@/features/services/components/platform-subdomain-section";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";
import { DeleteServiceCard } from "@/features/services/components/delete-service-card";
import { isCron } from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: ServiceSettingsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Settings · bex dashboard` }],
  }),
});

/**
 * The Settings tab (w5/m7, w1/m11.5, w5/m13): the Instance Type section
 * Render's settings page leads with, then Build & Deploy (repo-backed Apps
 * only — Source/Branch read-only, Root Directory editable), Custom Domains,
 * and the platform subdomain (Render parity). Name/region are future
 * milestones.
 */
export function ServiceSettingsPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId);
  const { t } = useTranslations();
  const cron = service ? isCron(service) : false;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("services.settingsTitle")}</CardTitle>
          <CardDescription>{t("services.settingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {!service && loading ? (
            <Skeleton className="h-10 w-full" />
          ) : (
            <div className="space-y-6">
              <InstanceTypeRow
                serviceId={serviceId}
                plan={service?.plan ?? null}
              />
              {/* Idle timeout only applies to an HTTP-served service — a cron_job
                  has no idle traffic to sleep on (Render parity, w5/m11). */}
              {!cron && (
                <IdleTimeoutRow
                  serviceId={serviceId}
                  plan={service?.plan ?? null}
                  idleTTLSeconds={service?.idleTTLSeconds ?? 0}
                />
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {cron ? (
        <CronDeploySection
          schedule={service?.schedule ?? null}
          command={service?.command ?? null}
        />
      ) : (
        <>
          {service?.repo && (
            <BuildDeploySection
              serviceId={serviceId}
              repo={service.repo}
              branch={service.branch}
              rootDir={service.rootDir}
            />
          )}
          <CustomDomainsSection serviceId={serviceId} />
          <PlatformSubdomainSection url={service?.url ?? null} />
        </>
      )}

      {/* Danger zone: type-to-confirm delete (every service type). Only once the
          service has loaded — the confirm matches against its exact name. */}
      {service && <DeleteServiceCard service={service} />}
    </div>
  );
}
