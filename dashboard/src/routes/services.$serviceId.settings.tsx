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

export const Route = createFileRoute("/services/$serviceId/settings")({
  component: ServiceSettingsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Settings · bex dashboard` }],
  }),
});

/**
 * The Settings tab (w5/m7): today, only the Instance Type section Render's
 * settings page leads with (captured live). Other sections (name/region/
 * build/deploy) are future milestones.
 */
export function ServiceSettingsPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId);
  const { t } = useTranslations();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.settingsTitle")}</CardTitle>
        <CardDescription>{t("services.settingsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {!service && loading ? (
          <Skeleton className="h-10 w-full" />
        ) : (
          <InstanceTypeRow serviceId={serviceId} plan={service?.plan ?? null} />
        )}
      </CardContent>
    </Card>
  );
}
