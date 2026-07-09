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
import { CustomDomainsSection } from "@/features/services/components/custom-domains-section";
import { PlatformSubdomainSection } from "@/features/services/components/platform-subdomain-section";

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: ServiceSettingsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Settings · bex dashboard` }],
  }),
});

/**
 * The Settings tab (w5/m7, w1/m11.5): the Instance Type section Render's settings
 * page leads with, then Custom Domains + the platform subdomain (Render parity).
 * Other sections (name/region/build/deploy) are future milestones.
 */
export function ServiceSettingsPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId);
  const { t } = useTranslations();

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
              <IdleTimeoutRow
                serviceId={serviceId}
                plan={service?.plan ?? null}
                idleTTLSeconds={service?.idleTTLSeconds ?? 0}
              />
            </div>
          )}
        </CardContent>
      </Card>

      <CustomDomainsSection serviceId={serviceId} />

      <PlatformSubdomainSection url={service?.url ?? null} />
    </div>
  );
}
