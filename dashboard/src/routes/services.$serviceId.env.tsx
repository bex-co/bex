import { createFileRoute } from "@tanstack/react-router";
import { EnvVarsPanel } from "@/features/services/components/env-vars-panel";

export const Route = createFileRoute("/services/$serviceId/env")({
  component: ServiceEnvPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Environment · bex dashboard` }],
  }),
});

// The Environment tab (w4/m6.5): a Render-style env-var surface over bex-api's
// env-vars GraphQL — keys list, per-key value reveal, add/update/delete.
export function ServiceEnvPage() {
  const { serviceId } = Route.useParams();
  return <EnvVarsPanel serviceId={serviceId} />;
}
