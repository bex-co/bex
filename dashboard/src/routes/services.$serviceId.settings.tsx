import { useMemo } from "react";
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
import {
  isCron,
  isStaticSite,
  isWorker,
} from "@/features/services/lib/service-type";
import type { en } from "@/i18n";

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
 *
 * Render parity (captured live from dashboard.render.com/web/.../settings):
 * a right-side sticky "Table of contents" nav, one anchor per section, while
 * the section cards themselves take the full remaining width instead of the
 * narrow centered `max-w-4xl` every other tab uses (opted out in the shared
 * `services.$serviceId.tsx` layout for this one route). The nav only lists
 * sections bex actually renders for this service's type — never a link to a
 * Render section (PR Previews, Log Stream, Maintenance Mode…) bex doesn't
 * have, which would 404 on click.
 */
export function ServiceSettingsPage() {
  const { serviceId } = Route.useParams();
  const { service, loading, refetch } = useServer(serviceId);
  const { t } = useTranslations();
  const cron = service ? isCron(service) : false;
  const staticSite = service ? isStaticSite(service) : false;
  const worker = service ? isWorker(service) : false;
  const repoBacked = !!service?.repo;

  const sections = useMemo(() => {
    const list: { id: string; labelKey: keyof typeof en }[] = [
      { id: "general", labelKey: "services.settingsNavGeneral" },
    ];
    if (cron) {
      list.push({ id: "deploy", labelKey: "services.deployTitle" });
      if (repoBacked) {
        list.push({ id: "build", labelKey: "services.buildDeployTitle" });
      }
    } else {
      if (repoBacked) {
        list.push({ id: "build", labelKey: "services.buildDeployTitle" });
      }
      if (staticSite) {
        list.push({ id: "static-site", labelKey: "services.staticTitle" });
      }
      if (!worker && !staticSite) {
        list.push({
          id: "health-checks",
          labelKey: "services.settingsHealthChecksTitle",
        });
      }
      list.push({ id: "custom-domains", labelKey: "services.domainsTitle" });
    }
    list.push({ id: "delete", labelKey: "services.dangerZoneTitle" });
    return list;
  }, [cron, staticSite, worker, repoBacked]);

  return (
    <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_200px]">
      <div className="min-w-0 space-y-6">
        <Card id="general">
          <CardHeader>
            <CardTitle>{t("services.settingsTitle")}</CardTitle>
            <CardDescription>
              {t("services.settingsDescription")}
            </CardDescription>
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
          <>
            <div id="deploy">
              <CronDeploySection
                serviceId={serviceId}
                schedule={service?.schedule ?? null}
                command={service?.command ?? null}
              />
            </div>
            {/* A git-sourced cron job still builds from a repo, so it keeps the
                Build & Deploy section — Root Directory + the Auto Deploy toggle,
                whose setAutoDeploy path is type-agnostic (w2/m9, w5/010). An
                image-backed cron has nothing to build, so it renders neither.
                Build Command / Log Stream stay deferred (ADR018 cron row). */}
            {service?.repo && (
              <div id="build">
                <BuildDeploySection
                  serviceId={serviceId}
                  repo={service.repo}
                  branch={service.branch}
                  rootDir={service.rootDir}
                  autoDeploy={service.autoDeploy ?? false}
                />
              </div>
            )}
          </>
        ) : (
          <>
            {service?.repo && (
              <div id="build">
                <BuildDeploySection
                  serviceId={serviceId}
                  repo={service.repo}
                  branch={service.branch}
                  rootDir={service.rootDir}
                  autoDeploy={service.autoDeploy ?? false}
                />
              </div>
            )}
            {staticSite && service && (
              <div id="static-site">
                <StaticSiteSection
                  serviceId={serviceId}
                  service={service}
                  refetch={refetch}
                />
              </div>
            )}
            {/* Health Checks: own section (Render parity — Render places this
                under a dedicated "Health Checks" heading in Settings, separate
                from the General card). Only web_service/private_service receive
                HTTP traffic and can have a ReadinessProbe path. */}
            {!worker && !staticSite && (
              <Card id="health-checks">
                <CardHeader>
                  <CardTitle>
                    {t("services.settingsHealthChecksTitle")}
                  </CardTitle>
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
            <div id="custom-domains" className="space-y-6">
              <CustomDomainsSection serviceId={serviceId} />
              <PlatformSubdomainSection url={service?.url ?? null} />
            </div>
          </>
        )}

        {/* Danger zone: type-to-confirm delete (every service type). Only once the
            service has loaded — the confirm matches against its exact name. */}
        {service && (
          <div id="delete">
            <DeleteServiceCard service={service} />
          </div>
        )}
      </div>

      <nav
        aria-label={t("services.settingsNavLabel")}
        className="hidden lg:block"
      >
        <div className="sticky top-6 space-y-1">
          {sections.map((section) => (
            <a
              key={section.id}
              href={`#${section.id}`}
              className="text-muted-foreground hover:bg-accent hover:text-foreground block rounded-md px-2 py-1.5 text-sm transition-colors"
            >
              {t(section.labelKey)}
            </a>
          ))}
        </div>
      </nav>
    </div>
  );
}
