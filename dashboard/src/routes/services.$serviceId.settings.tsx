import { createFileRoute, useRouter } from "@tanstack/react-router";
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
import { CustomDomainsSection } from "@/features/services/components/custom-domains-section";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";
import { DeleteServiceCard } from "@/features/services/components/delete-service-card";
import { SuspendServiceCard } from "@/features/services/components/suspend-service-card";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { StaticSiteSection } from "@/features/services/components/static-site-section";
import { HealthCheckPathRow } from "@/features/services/components/health-check-path-row";
import { ServiceNotificationsRow } from "@/features/services/components/service-notifications-row";
import { DisplayNameRow } from "@/features/services/components/display-name-row";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import { DeployHookSection } from "@/features/services/components/deploy-hook-section";
import { MaxShutdownDelayRow } from "@/features/services/components/max-shutdown-delay-row";
import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";
import { MaintenanceModeSection } from "@/features/services/components/maintenance-mode-section";
import { RegistryCredentialSection } from "@/features/services/components/registry-credential-section";
import {
  ServiceSettingsNavigation,
  type ServiceSettingsSection,
} from "@/features/services/components/service-settings-navigation";
import {
  isCron,
  isStaticSite,
  isWebService,
  isWorker,
  supportsMaxShutdownDelay,
} from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceSettingsPage serviceId={serviceId} />;
}

/**
 * The Settings tab (w5/m7, w1/m11.5, w5/m13): the mutable service label and
 * Instance Type section Render's settings page leads with, then Build & Deploy
 * (repo-backed Apps only — Source/Branch read-only, Root Directory editable),
 * Custom Domains, and the platform subdomain (Render parity).
 */
export function ServiceSettingsPage({ serviceId }: { serviceId: string }) {
  const { service, loading, refetch } = useServer(serviceId, { poll: false });
  const router = useRouter();
  const { pending, run } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();
  const cron = service ? isCron(service) : false;
  const staticSite = service ? isStaticSite(service) : false;
  const worker = service ? isWorker(service) : false;
  // A Dockerfile build (docker runtime, or the legacy dockerfile builder) builds
  // from a Dockerfile, not a Build Command — Render shows Dockerfile Path there
  // instead. Every other repo-backed build is native and carries a Build Command.
  const dockerBuild =
    service?.runtime === "docker" ||
    (!service?.runtime && service?.builder === "dockerfile");
  const registryCredentialEligible =
    service != null && !staticSite && (!service.repo || dockerBuild);
  const navigationSections: ServiceSettingsSection[] = ["general"];
  if (cron) navigationSections.push("deploy");
  if (service?.repo) navigationSections.push("build");
  if (staticSite && service) navigationSections.push("static-site");
  if (!cron) navigationSections.push("domains", "networking");
  if (registryCredentialEligible)
    navigationSections.push("registry-credential");
  navigationSections.push("notifications");
  if (!cron && !worker && !staticSite) navigationSections.push("health-checks");
  if (service && isWebService(service)) navigationSections.push("maintenance");
  if (cron || !service?.repo) navigationSections.push("deploy-hook");
  if (service) {
    navigationSections.push(service.suspended ? "resume" : "suspend");
    navigationSections.push("danger-zone");
  }

  return (
    <div className="service-settings-layout grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
      <ServiceSettingsNavigation
        sections={navigationSections}
        className="sticky top-0 z-20 -mx-4 border-y bg-background/95 px-4 py-2 backdrop-blur sm:-mx-6 sm:px-6 lg:top-6 lg:col-start-2 lg:row-start-1 lg:mx-0 lg:border-0 lg:bg-transparent lg:px-0 lg:py-0 lg:backdrop-blur-none"
      />

      <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
        <section id="general" className="scroll-mt-6">
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
                  {/* Idle timeout only applies to running-container services — a
                  cron_job has no idle traffic to sleep on, and a static_site
                  serves from the object store with no pod to hibernate
                  (Render parity, w5/m11, w1/m21). Manual instance count lives
                  on the Scaling tab beside autoscaling (w7/m43 — Render's
                  placement; supersedes the w5/m16 Settings stepper). */}
                  {!cron && !staticSite && (
                    <>
                      <IdleTimeoutRow
                        serviceId={serviceId}
                        plan={service?.plan ?? null}
                        idleTTLSeconds={service?.idleTTLSeconds ?? 0}
                      />
                      {service && supportsMaxShutdownDelay(service) && (
                        <MaxShutdownDelayRow
                          serviceId={serviceId}
                          maxShutdownDelaySeconds={
                            service.maxShutdownDelaySeconds
                          }
                          onChanged={() => void refetch()}
                        />
                      )}
                    </>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </section>

        {cron ? (
          <>
            <section id="deploy" className="scroll-mt-6">
              <CronDeploySection
                serviceId={serviceId}
                schedule={service?.schedule ?? null}
                command={service?.command ?? null}
              />
            </section>
            {/* A git-sourced cron job still builds from a repo, so it keeps the
                Build & Deploy section — Root Directory + the Auto Deploy toggle,
                whose setAutoDeploy path is type-agnostic (w2/m9, w5/010). An
                image-backed cron has nothing to build, so it renders neither.
                Build Command / Log Stream stay deferred (ADR018 cron row). */}
            {service?.repo && (
              <section id="build" className="scroll-mt-6 space-y-6">
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
                  // Cron's deploy concerns live in its own Deploy (Schedule/Command)
                  // section, so no separate Deploy card — Auto-Deploy folds into
                  // Build and the Deploy Hook stays a standalone card below (w5/m52).
                  showDeployCard={false}
                />
              </section>
            )}
          </>
        ) : (
          <>
            {service?.repo && (
              <section id="build" className="scroll-mt-6 space-y-6">
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
                  preDeployCommand={service.preDeployCommand}
                  // Pre-Deploy Command applies to web/private/worker; a static_site
                  // has no running container, so hide the field for it (w1/m33).
                  showPreDeployCommand={!staticSite}
                  // Build Command shows for every native build — static sites (w7/m41)
                  // and native-runtime web/private/worker services (w5/m51). A
                  // Dockerfile build shows Dockerfile Path instead.
                  showBuildCommand={!dockerBuild}
                  showStartCommand={!staticSite}
                  showDockerfilePath={!staticSite}
                />
              </section>
            )}
            {staticSite && service && (
              <section id="static-site" className="scroll-mt-6">
                <StaticSiteSection
                  serviceId={serviceId}
                  service={service}
                  refetch={refetch}
                />
              </section>
            )}
            {/* Custom Domains, with the platform-subdomain toggle folded in at the
                bottom of the card (Render parity, w5/m52). */}
            <section id="domains" className="scroll-mt-6">
              <CustomDomainsSection
                serviceId={serviceId}
                subdomain={{
                  url: service?.url ?? null,
                  renderSubdomainPolicy: service?.renderSubdomainPolicy,
                }}
              />
            </section>
            {/* Networking (w7/m32): inbound IP allowlist — web_service and
                static_site only (both have a public Ingress). */}
            <section id="networking" className="scroll-mt-6">
              <ServiceNetworkingPanel
                serviceId={serviceId}
                currentAllowList={service?.ipAllowListEntries}
                onSaved={refetch}
              />
            </section>
          </>
        )}

        {registryCredentialEligible ? (
          <section id="registry-credential" className="scroll-mt-6">
            <RegistryCredentialSection
              key={serviceId}
              serviceId={serviceId}
              registryCredentialId={service.registryCredentialId}
              onChanged={() => void refetch()}
            />
          </section>
        ) : null}

        {/* Notifications (w4/m21): the per-service deploy-failure override applies
            to every service type (Render places it at the service level, not
            gated by type), so it renders outside the cron/else branches above. */}
        <section id="notifications" className="scroll-mt-6">
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
        </section>

        {/* Health Checks (Render places this section after Notifications, w5/m52):
            the HTTP path bex polls before routing traffic. web_service /
            private_service only — never cron/worker/static (no HTTP readiness). */}
        {!cron && !worker && !staticSite && (
          <section id="health-checks" className="scroll-mt-6">
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
          </section>
        )}

        {/* Maintenance Mode (w1/m37): web_service only, matching the backend's
            requireWebService guard. */}
        {service && isWebService(service) && (
          <section id="maintenance" className="scroll-mt-6">
            <MaintenanceModeSection
              serviceId={serviceId}
              serviceName={service.name}
              plan={service.plan}
              maintenanceMode={service.maintenanceMode}
            />
          </section>
        )}

        {/* Deploy Hook: embedded inside the Deploy card for a repo-backed
            non-cron service (w5/m52). It stays a standalone card only when there's
            no Deploy card to hold it — a cron_job or an image-backed service. */}
        {(cron || !service?.repo) && (
          <section id="deploy-hook" className="scroll-mt-6">
            <DeployHookSection serviceId={serviceId} />
          </section>
        )}

        {/* Suspend / Resume: mirrors Render's bottom-of-settings placement.
            Only once the service has loaded so we know its suspended state. */}
        {service && (
          <section id="suspend" className="scroll-mt-6">
            <SuspendServiceCard
              service={service}
              pending={pending?.id === service.id ? pending.action : null}
              onRun={run}
            />
          </section>
        )}

        {/* Danger zone: type-to-confirm delete (every service type). Only once the
            service has loaded — the confirm matches against its immutable id. */}
        {service && (
          <section id="danger-zone" className="scroll-mt-6">
            <DeleteServiceCard service={service} />
          </section>
        )}
      </div>
    </div>
  );
}
