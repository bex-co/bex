import { ServiceTabDescription } from "@/features/services/components/service-tab-description";
import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { ServiceEnvironmentEditor } from "@/features/services/components/service-environment-editor";
import { EnvGroupsPanel } from "@/features/services/components/env-groups-panel";
import { ServiceEnvironmentSkeleton } from "@/common/components/route-skeletons";
import { useServer } from "@/features/services/hooks/use-server";
import { isDockerBuild } from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/env")({
  head: serviceTabHead("services.navEnvironment", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceEnvironmentSkeleton,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceEnvPage key={serviceId} serviceId={serviceId} />;
}

// The Environment tab (w4/m6.5): a Render-style environment surface over bex-api.
// Three stacked sections — env vars (keys list + per-key reveal + CRUD), secret
// files (per-service, same reveal/CRUD shape), and environment groups (reusable
// bundles linked to this service). Creating a group lives in the groups card,
// beside the groups it adds to; a second Create group above the variables
// pushed them below the first screen on a phone.
export function ServiceEnvPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  // Reads the detail layout's cached document; the layout owns polling.
  const { service } = useServer(serviceId, { poll: false });
  return (
    <div className="space-y-6">
      {/* The tab's description, under the layout's heading. */}
      <ServiceTabDescription>
        {t("services.environmentPageDescription")}
      </ServiceTabDescription>
      <ServiceEnvironmentEditor
        serviceId={serviceId}
        suspended={service?.suspended ?? false}
        dockerBuild={!!service && isDockerBuild(service)}
      />
      <EnvGroupsPanel serviceId={serviceId} />
    </div>
  );
}
