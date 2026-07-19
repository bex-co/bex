import { createFileRoute } from "@tanstack/react-router";
import { StaticEdgeRulePage } from "@/features/services/components/static-edge-rule-page";
import { HeadersEditor } from "@/features/services/components/static-site-section";

export const Route = createFileRoute("/services/$serviceId/headers")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceHeadersPage serviceId={serviceId} />;
}

/** The dedicated Headers page (w5/m48/t002) — see StaticEdgeRulePage. */
export function ServiceHeadersPage({ serviceId }: { serviceId: string }) {
  return (
    <StaticEdgeRulePage
      serviceId={serviceId}
      titleKey="services.navHeaders"
      renderEditor={(service, { setHeaders, busy }) => (
        <HeadersEditor
          key={service.id}
          headers={service.headers}
          onSave={setHeaders}
          busy={busy}
        />
      )}
    />
  );
}
