import { createFileRoute } from "@tanstack/react-router";
import { EnvVarsPanel } from "@/features/services/components/env-vars-panel";
import { SecretFilesPanel } from "@/features/services/components/secret-files-panel";
import { EnvGroupsPanel } from "@/features/services/components/env-groups-panel";

export const Route = createFileRoute("/services/$serviceId/env")({
  component: ServiceEnvPage,
});

// The Environment tab (w4/m6.5): a Render-style environment surface over bex-api.
// Three stacked sections — env vars (keys list + per-key reveal + CRUD), secret
// files (per-service, same reveal/CRUD shape), and environment groups (reusable
// bundles linked to this service).
export function ServiceEnvPage() {
  const { serviceId } = Route.useParams();
  return (
    <div className="space-y-6">
      <EnvVarsPanel serviceId={serviceId} />
      <SecretFilesPanel serviceId={serviceId} />
      <EnvGroupsPanel serviceId={serviceId} />
    </div>
  );
}
