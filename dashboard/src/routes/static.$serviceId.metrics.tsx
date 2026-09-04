import { createFileRoute } from "@tanstack/react-router";
import { ServiceMetricsPage } from "./services.$serviceId.metrics";
import {
  DEFAULT_RANGE_PRESET,
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
} from "@/features/metrics/lib/range";
import { ServiceMetricsSkeleton } from "@/common/components/route-skeletons";

/** Static-site Metrics tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/metrics")({
  component: RouteComponent,
  pendingComponent: StaticMetricsPending,
  // Same URL-persisted range as services.$serviceId.metrics (w6/065).
  validateSearch: parseRangeSearch,
});

function StaticMetricsPending() {
  return <ServiceMetricsSkeleton staticSite />;
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <ServiceMetricsPage
      serviceId={serviceId}
      range={rangeFromSearch(search, DEFAULT_RANGE_PRESET)}
      onRangeChange={(range) =>
        // Functional form, deliberately: its return is committed verbatim, so
        // rangeToSearch's explicit undefineds actually clear stale custom
        // bounds (the w7/m42 lesson from the Logs route).
        void navigate({ search: () => rangeToSearch(range), replace: true })
      }
    />
  );
}
