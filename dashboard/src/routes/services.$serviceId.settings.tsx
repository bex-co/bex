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
import { HealthCheckPathRow } from "@/features/services/components/health-check-path-row";
import { ServiceNotificationsRow } from "@/features/services/components/service-notifications-row";
import { DisplayNameRow } from "@/features/services/components/display-name-row";
import { DeployHookSection } from "@/features/services/components/deploy-hook-section";
import { MaxShutdownDelayRow } from "@/features/services/components/max-shutdown-delay-row";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";
import { MaintenanceModeSection } from "@/features/services/components/maintenance-mode-section";
import {
  isCron,
  isStaticSite,
  isWebService,
  isWorker,
  supportsMaxShutdownDelay,
} from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: ServiceSettingsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Settings · bex dashboard` }],
  }),
});

/**
 * The Settings tab (w5/m7, w1/m11.5, w5/m13): the mutable service label and
 * Instance Type section Render's settings page leads with, then Build & Deploy
 * (repo-backed Apps only — Source/Branch read-only, Root Directory editable),
 * Custom Domains, and the platform subdomain (Render parity).
 */
export function ServiceSettingsPage() {
  const { serviceId } = Route.useParams();
  const { service, loading, refetch } = useServer(serviceId);
  const { t } = useTranslations();
  const cron = service ? isCron(service) : false;
  const staticSite = service ? isStaticSite(service) : false;
  const worker = service ? isWorker(service) : false;

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
              <DisplayNameRow
                serviceId={serviceId}
                displayName={service?.displayName}
                onChanged={() => void refetch()}
              />
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
                  {service && supportsMaxShutdownDelay(service) && (
                    <MaxShutdownDelayRow
                      serviceId={serviceId}
                      maxShutdownDelaySeconds={service.maxShutdownDelaySeconds}
                      onChanged={() => void refetch()}
                    />
                  )}
                </>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {cron ? (
        <>
          <CronDeploySection
            serviceId={serviceId}
            schedule={service?.schedule ?? null}
            command={service?.command ?? null}
          />
          {/* A git-sourced cron job still builds from a repo, so it keeps the
              Build & Deploy section — Root Directory + the Auto Deploy toggle,
              whose setAutoDeploy path is type-agnostic (w2/m9, w5/010). An
              image-backed cron has nothing to build, so it renders neither.
              Build Command / Log Stream stay deferred (ADR018 cron row). */}
          {service?.repo && (
            <BuildDeploySection
              serviceId={serviceId}
              repo={service.repo}
              branch={service.branch}
              rootDir={service.rootDir}
              runtime={service.runtime}
              builder={service.builder}
              startCommand={service.startCommand}
              dockerfilePath={service.dockerfilePath}
              buildFilter={service.buildFilter}
              autoDeploy={service.autoDeploy ?? false}
              preDeployCommand={service.preDeployCommand}
              // A cron_job runs its own Command; the pre-deploy step doesn't
              // apply (the backend rejects it), so hide the field here.
              showPreDeployCommand={false}
              showStartCommand={false}
              showDockerfilePath={false}
            />
          )}
        </>
      ) : (
        <>
          {service?.repo && (
            <BuildDeploySection
              serviceId={serviceId}
              repo={service.repo}
              branch={service.branch}
              rootDir={service.rootDir}
              runtime={service.runtime}
              builder={service.builder}
              startCommand={service.startCommand}
              dockerfilePath={service.dockerfilePath}
              buildFilter={service.buildFilter}
              autoDeploy={service.autoDeploy ?? false}
              preDeployCommand={service.preDeployCommand}
              // Pre-Deploy Command applies to web/private/worker; a static_site
              // has no running container, so hide the field for it (w1/m33).
              showPreDeployCommand={!staticSite}
              showStartCommand={!staticSite}
              showDockerfilePath={!staticSite}
            />
          )}
          {staticSite && service && (
            <StaticSiteSection
              serviceId={serviceId}
              service={service}
              refetch={refetch}
            />
          )}
          {/* Health Checks: own section (Render parity — Render places this
              under a dedicated "Health Checks" heading in Settings, separate
              from the General card). Only web_service/private_service receive
              HTTP traffic and can have a ReadinessProbe path. */}
          {!worker && !staticSite && (
            <Card>
              <CardHeader>
                <CardTitle>{t("services.settingsHealthChecksTitle")}</CardTitle>
                <CardDescription>
                  {t("services.settingsHealthChecksDescription")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <HealthCheckPathRow
                  serviceId={serviceId}
                  healthCheckPath={service?.healthCheckPath}
                />
              </CardContent>
            </Card>
          )}
          <CustomDomainsSection serviceId={serviceId} />
          <PlatformSubdomainSection
            serviceId={serviceId}
            url={service?.url ?? null}
            renderSubdomainPolicy={service?.renderSubdomainPolicy}
          />
          {/* Networking (w7/m32): inbound IP allowlist — web_service and
              static_site only (both have a public Ingress). */}
          <ServiceNetworkingPanel
            serviceId={serviceId}
            currentAllowList={service?.ipAllowList}
            onSaved={refetch}
          />
          {/* Maintenance Mode (w1/m37): web_service only, matching the
              backend's requireWebService guard. */}
          {service && isWebService(service) && (
            <MaintenanceModeSection
              serviceId={serviceId}
              serviceName={service.name}
              plan={service.plan}
              maintenanceMode={service.maintenanceMode}
            />
          )}
        </>
      )}

      {/* Notifications (w4/m21): the per-service deploy-failure override applies
          to every service type (Render places it at the service level, not
          gated by type), so it renders outside the cron/else branches above. */}
      <Card>
        <CardHeader>
          <CardTitle>{t("services.settingsNotificationsTitle")}</CardTitle>
          <CardDescription>
            {t("services.settingsNotificationsDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ServiceNotificationsRow
            serviceId={serviceId}
            notificationsToSend={service?.notificationsToSend}
          />
        </CardContent>
      </Card>

      <DeployHookSection serviceId={serviceId} />

      {/* Danger zone: type-to-confirm delete (every service type). Only once the
          service has loaded — the confirm matches against its immutable id. */}
      {service && <DeleteServiceCard service={service} />}
    </div>
  );
}
