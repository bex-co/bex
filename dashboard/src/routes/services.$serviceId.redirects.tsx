import { createFileRoute } from "@tanstack/react-router";
import { StaticEdgeRulePage } from "@/features/services/components/static-edge-rule-page";
import { RoutesEditor } from "@/features/services/components/static-site-section";
import { StaticEdgeRulesSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/redirects")({
  component: RouteComponent,
  pendingComponent: StaticEdgeRulesSkeleton,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceRedirectsPage serviceId={serviceId} />;
}

/** The dedicated Redirects/Rewrites page (w5/m48/t002) — see StaticEdgeRulePage. */
export function ServiceRedirectsPage({ serviceId }: { serviceId: string }) {
  return (
    <StaticEdgeRulePage
      serviceId={serviceId}
      titleKey="services.navRedirects"
      renderEditor={(service, { setRoutes, busy }) => (
        <RoutesEditor
          key={service.id}
          routes={service.routes}
          onSave={setRoutes}
          busy={busy}
        />
      )}
    />
  );
}
