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
import { StaticSiteSection } from "@/features/services/components/static-site-section";
import { ScalingRow } from "@/features/services/components/scaling-row";
import { isCron, isStaticSite } from "@/features/services/lib/service-type";

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
  const { service, loading, refetch } = useServer(serviceId);
  const { t } = useTranslations();
  const cron = service ? isCron(service) : false;
  const staticSite = service ? isStaticSite(service) : false;

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
              {/* Idle timeout and manual scaling only apply to running-container
                  services — a cron_job has no idle traffic to sleep on, and a
                  static_site serves from the object store with no pod to
                  hibernate or scale (Render parity, w5/m11, w1/m21, w5/m16). */}
              {!cron && !staticSite && (
                <>
                  <IdleTimeoutRow
                    serviceId={serviceId}
                    plan={service?.plan ?? null}
                    idleTTLSeconds={service?.idleTTLSeconds ?? 0}
                  />
                  <ScalingRow
                    serviceId={serviceId}
                    replicas={service?.replicas ?? null}
                  />
                </>
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
              autoDeploy={service.autoDeploy ?? false}
            />
          )}
          {staticSite && service && (
            <StaticSiteSection
              serviceId={serviceId}
              service={service}
              refetch={refetch}
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
