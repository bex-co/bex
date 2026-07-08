import { createFileRoute } from "@tanstack/react-router";
import { LogViewer } from "@/features/logs/components/log-viewer";

export const Route = createFileRoute("/services/$serviceId/logs")({
  component: RouteComponent,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Logs · bex dashboard` }],
  }),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceLogsPage serviceId={serviceId} />;
}

// The Logs tab — bex-api's historical logs query + SSE live tail, laid out like
// Render's Logs viewer (w5/m6). The service chrome (header + nav + content
// container) comes from the `services.$serviceId` layout route; this renders
// only the viewer into the shared `<Outlet/>`. Exported taking `serviceId` as a
// prop (like the Overview page) so the routing test can mount it under a
// reconstructed router without the file Route's param context.
export function ServiceLogsPage({ serviceId }: { serviceId: string }) {
  return <LogViewer resource={serviceId} />;
}
