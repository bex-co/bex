import type { ReactNode } from "react";
import {
  createFileRoute,
  getRouteApi,
  useRouter,
} from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { FieldRowsSkeleton } from "@/common/components/detail-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { InstanceTypeRow } from "@/features/services/components/instance-type-row";
import { IdleTimeoutRow } from "@/features/services/components/idle-timeout-row";
import { BuildDeploySection } from "@/features/services/components/build-deploy-section";
import { DeployCard } from "@/features/services/components/deploy-card";
import { ServiceSourceCard } from "@/features/services/components/service-source-card";
import { CustomDomainsSection } from "@/features/services/components/custom-domains-section";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";
import { DeleteServiceCard } from "@/features/services/components/delete-service-card";
import { SuspendServiceCard } from "@/features/services/components/suspend-service-card";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { StaticSiteSection } from "@/features/services/components/static-site-section";
import { HealthCheckPathRow } from "@/features/services/components/health-check-path-row";
import { ServicePortRow } from "@/features/services/components/service-port-row";
import { ServiceNotificationsRow } from "@/features/services/components/service-notifications-row";
import { DisplayNameRow } from "@/features/services/components/display-name-row";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { DeployHookSection } from "@/features/services/components/deploy-hook-section";
import { MaxShutdownDelayRow } from "@/features/services/components/max-shutdown-delay-row";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";
import { ServiceOutboundIpsPanel } from "@/features/services/components/service-outbound-ips-panel";
import { MaintenanceModeSection } from "@/features/services/components/maintenance-mode-section";
import { RegistryCredentialSection } from "@/features/services/components/registry-credential-section";
import { ServiceSettingsNavigation } from "@/features/services/components/service-settings-navigation";
import {
  isCron,
  isDockerBuild,
  isStaticSite,
  supportsMaxShutdownDelay,
} from "@/features/services/lib/service-type";
import {
  serviceSettingsSections,
  type ServiceSettingsSectionId,
} from "@/features/services/lib/settings-sections";
import { toServiceView } from "@/features/services/lib/status";
import { ServiceSettingsSkeleton } from "@/common/components/route-skeletons";
import { SECTION_NAVIGATION_STICKY_CLASS } from "@/common/components/section-navigation";

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: RouteComponent,
  pendingComponent: ServiceSettingsPending,
});

function ServiceSettingsPending() {
  const parent = getRouteApi("/services/$serviceId").useLoaderData();
  return (
    <ServiceSettingsSkeleton
      service={
        parent?.state === "ready" ? toServiceView(parent.resource) : undefined
      }
    />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceSettingsPage serviceId={serviceId} />;
}

/**
 * The Settings tab (w5/m7, w1/m11.5, w5/m13): the mutable service label and
 * Instance Type section Render's settings page leads with, then Build & Deploy
 * (repo-backed Apps only — Source/Branch read-only, Root Directory editable),
 * Custom Domains, and the platform subdomain (Render parity). Which sections a
 * service has, and their order, is serviceSettingsSections: the navigation and
 * the pending skeleton draw the same list (w5/m124).
 */
export function ServiceSettingsPage({ serviceId }: { serviceId: string }) {
  const { service, loading, refetch } = useServer(serviceId, { poll: false });
  const router = useRouter();
  const { pending, run } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();
  const sections = serviceSettingsSections(service ?? undefined);
  const cron = service ? isCron(service) : false;
  const staticSite = service ? isStaticSite(service) : false;

  // A section's content. The list holds the sections that read the loaded
  // service only once it has loaded; the guards narrow its type.
  function renderSection(id: ServiceSettingsSectionId): ReactNode {
    switch (id) {
      case "general":
        return (
          <Card>
            <CardHeader>
              <CardTitle>{t("services.generalTitle")}</CardTitle>
              <CardDescription>
                {t("services.settingsDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!service && loading ? (
                <FieldRowsSkeleton rows={4} />
              ) : (
                <div className="space-y-6">
                  <DisplayNameRow
                    serviceId={serviceId}
                    displayName={service?.displayName}
                    name={service?.name}
                    onChanged={() => void router.invalidate()}
                  />
                  {/* Region: read-only platform placement (Render's General row),
                  projected from BEX_REGION (w1/m53). Hidden when the install
                  sets no region — never inferred. Permanently disabled (no
                  pencil): region is installation-stamped, not tenant-editable.
                  A static_site is region-agnostic — it serves from the object
                  store via the shared static-server (docs/ADR029-static-sites.md),
                  so Render's static Settings omits Region entirely (w5/m57/t003). */}
                  {service?.region && !staticSite && (
                    <EditableFieldRow
                      label={t("services.regionLabel")}
                      hint={t("services.regionHint")}
                      value={service.region}
                      editLabel={t("services.regionLabel")}
                      disabled
                      onSave={async () => false}
                    />
                  )}
                  {/* A static_site has no instance type — it serves from the object
                  store, not a sized pod (Render shows no Instance Type for
                  static sites; w5/m48/t004). The Plan tab is gated the same way. */}
                  {!staticSite && (
                    <InstanceTypeRow
                      serviceId={serviceId}
                      plan={service?.plan ?? null}
                    />
                  )}
                  {/* Auto-sleep is a public-web-only feature: the row owns the
                  eligibility predicate so every settings caller stays honest.
                  Manual instance count lives on the Scaling tab beside
                  autoscaling (w7/m43 — Render's placement). */}
                  {service && (
                    <IdleTimeoutRow
                      serviceId={serviceId}
                      serviceType={service.type}
                      plan={service.plan}
                      idleTTLSeconds={service.idleTTLSeconds ?? 0}
                    />
                  )}
                  {service && supportsMaxShutdownDelay(service) && (
                    <MaxShutdownDelayRow
                      serviceId={serviceId}
                      maxShutdownDelaySeconds={service.maxShutdownDelaySeconds}
                      onChanged={() => void refetch()}
                    />
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        );
      case "deploy":
        if (!service) return null;
        return cron ? (
          <CronDeploySection
            serviceId={serviceId}
            schedule={service.schedule ?? null}
            command={service.command ?? null}
          />
        ) : (
          // An Existing Image service has no build, so the Deploy card a
          // repo-backed service shows under Build stands alone (w4/m166).
          <DeployCard
            serviceId={serviceId}
            commandKind="image"
            command={service.startCommand ?? null}
            preDeployCommand={service.preDeployCommand ?? null}
            showPreDeployCommand
            showCommand
          />
        );
      case "source":
        if (!service) return null;
        return (
          <ServiceSourceCard
            serviceId={serviceId}
            repo={service.repo ?? null}
            branch={service.branch ?? null}
            imagePath={service.imagePath ?? null}
            registryCredentialId={service.registryCredentialId ?? null}
          />
        );
      case "build":
        if (!service?.repo) return null;
        return (
          <BuildDeploySection
            serviceId={serviceId}
            repo={service.repo}
            branch={service.branch}
            rootDir={service.rootDir}
            runtime={service.runtime}
            builder={service.builder}
            buildCommand={service.buildCommand}
            startCommand={service.startCommand}
            dockerfilePath={service.dockerfilePath}
            buildFilter={service.buildFilter}
            autoDeploy={service.autoDeploy ?? false}
            pushDeliveryMethod={service.pushDeliveryMethod}
            preDeployCommand={service.preDeployCommand}
            // A cron job runs its own Command and keeps its deploy concerns in
            // its Deploy (Schedule/Command) section: no Deploy card, so
            // Auto-Deploy folds into Build and its hook stands alone (w5/m52),
            // and its source stays inline (w2/m9, w5/010). A static site runs
            // no container, so no Pre-Deploy or start command (w1/m33).
            showDeployCard={!cron}
            showSourceFields={cron}
            showPreDeployCommand={!cron && !staticSite}
            showStartCommand={!cron && !staticSite}
            showDockerfilePath={!cron && !staticSite}
            // Build Command shows for every native build — static sites (w7/m41)
            // and native-runtime web/private/worker services (w5/m51). A
            // Dockerfile build shows Dockerfile Path instead.
            showBuildCommand={!cron && !isDockerBuild(service)}
          />
        );
      case "static-site":
        if (!service) return null;
        return (
          <StaticSiteSection
            serviceId={serviceId}
            service={service}
            refetch={refetch}
          />
        );
      case "domains":
        // The platform-subdomain toggle folds into the bottom of the card
        // (Render parity, w5/m52).
        return (
          <CustomDomainsSection
            serviceId={serviceId}
            subdomain={{
              url: service?.url ?? null,
              renderSubdomainPolicy: service?.renderSubdomainPolicy,
            }}
          />
        );
      case "networking":
        return (
          <ServiceNetworkingPanel
            serviceId={serviceId}
            currentAllowList={service?.ipAllowListEntries}
            proxiedDomains={service?.ipAllowListProxiedDomains}
            onSaved={refetch}
          />
        );
      case "outbound-ips":
        if (!service) return null;
        return <ServiceOutboundIpsPanel ips={service.outboundIps?.ips} />;
      case "registry-credential":
        if (!service) return null;
        return (
          <RegistryCredentialSection
            key={serviceId}
            serviceId={serviceId}
            registryCredentialId={service.registryCredentialId}
            onChanged={() => void refetch()}
          />
        );
      case "notifications":
        // The per-service deploy-failure override (w4/m21).
        return (
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
        );
      case "port":
        // The container port bex routes to and injects as $PORT — the setting
        // the reserved-PORT refusal names (w4/m121/t003).
        return (
          <Card>
            <CardHeader>
              <CardTitle>{t("services.settingsPortTitle")}</CardTitle>
              <CardDescription>
                {t("services.settingsPortDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ServicePortRow serviceId={serviceId} port={service?.port} />
            </CardContent>
          </Card>
        );
      case "health-checks":
        // The HTTP path bex polls before routing traffic (w5/m52).
        return (
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
        );
      case "maintenance":
        if (!service) return null;
        return (
          <MaintenanceModeSection
            serviceId={serviceId}
            serviceName={service.name}
            plan={service.plan}
            maintenanceMode={service.maintenanceMode}
          />
        );
      case "deploy-hook":
        return <DeployHookSection serviceId={serviceId} />;
      case "suspend":
        if (!service) return null;
        return (
          <SuspendServiceCard
            service={service}
            pending={pending?.id === service.id ? pending.action : null}
            onRun={run}
          />
        );
      case "danger-zone":
        // Type-to-confirm delete: the confirm matches the immutable id.
        if (!service) return null;
        return <DeleteServiceCard service={service} />;
      default: {
        const unrendered: never = id;
        return unrendered;
      }
    }
  }

  return (
    <div className="service-settings-layout grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
      <ServiceSettingsNavigation
        sections={sections}
        suspended={service?.suspended ?? false}
        className={SECTION_NAVIGATION_STICKY_CLASS}
      />

      <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
        {sections.map((id) => (
          <section
            key={id}
            id={id}
            className={id === "build" ? "scroll-mt-6 space-y-6" : "scroll-mt-6"}
          >
            {renderSection(id)}
          </section>
        ))}
      </div>
    </div>
  );
}
