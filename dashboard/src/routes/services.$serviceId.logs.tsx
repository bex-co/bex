import { createFileRoute } from "@tanstack/react-router";
import { LogViewer } from "@/features/logs/components/log-viewer";
import {
  DEFAULT_RANGE_PRESET,
  type RangePreset,
} from "@/features/metrics/lib/range";
import {
  logRangeFromSearch,
  parseLogSearch,
} from "@/features/logs/lib/log-search";

export const Route = createFileRoute("/services/$serviceId/logs")({
  component: RouteComponent,
  validateSearch: parseLogSearch,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Logs · bex dashboard` }],
  }),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const { range: rangeID } = Route.useSearch();
  const navigate = Route.useNavigate();
  const range = logRangeFromSearch({ range: rangeID });
  return (
    <ServiceLogsPage
      serviceId={serviceId}
      range={range}
      onRangeChange={(next) =>
        void navigate({ search: { range: next.id }, replace: true })
      }
    />
  );
}

// The Logs tab — bex-api's historical logs query + SSE live tail, laid out like
// Render's Logs viewer (w5/m6). The service chrome (header + nav + content
// container) comes from the `services.$serviceId` layout route; this renders
// only the viewer into the shared `<Outlet/>`. Exported taking `serviceId` as a
// prop (like the Overview page) so the routing test can mount it under a
// reconstructed router without the file Route's param context.
export function ServiceLogsPage({
  serviceId,
  range = DEFAULT_RANGE_PRESET,
  onRangeChange = () => undefined,
}: {
  serviceId: string;
  range?: RangePreset;
  onRangeChange?: (range: RangePreset) => void;
}) {
  return (
    <LogViewer
      resource={serviceId}
      range={range}
      onRangeChange={onRangeChange}
    />
  );
}
