import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { ServiceEnvironmentEditor } from "@/features/services/components/service-environment-editor";
import { EnvGroupsPanel } from "@/features/services/components/env-groups-panel";
import { ServiceEnvironmentSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/env")({
  component: RouteComponent,
  pendingComponent: ServiceEnvironmentSkeleton,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceEnvPage serviceId={serviceId} />;
}

// The Environment tab (w4/m6.5): a Render-style environment surface over bex-api.
// Three stacked sections — env vars (keys list + per-key reveal + CRUD), secret
// files (per-service, same reveal/CRUD shape), and environment groups (reusable
// bundles linked to this service).
export function ServiceEnvPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const [createGroupOpen, setCreateGroupOpen] = useState(false);
  return (
    <div className="space-y-6">
      <div className="flex flex-col items-start gap-3 sm:flex-row sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">
            {t("services.environmentPageTitle")}
          </h1>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("services.environmentPageDescription")}
          </p>
        </div>
        <Button
          variant="outline"
          aria-haspopup="dialog"
          aria-expanded={createGroupOpen}
          onClick={() => setCreateGroupOpen(true)}
        >
          <Plus /> {t("services.envGroupCreate")}
        </Button>
      </div>
      <ServiceEnvironmentEditor serviceId={serviceId} />
      <EnvGroupsPanel
        serviceId={serviceId}
        createOpen={createGroupOpen}
        onCreateOpenChange={setCreateGroupOpen}
      />
    </div>
  );
}
