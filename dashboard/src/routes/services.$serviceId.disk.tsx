import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";

import { DiskSection } from "@/features/services/components/disk-section";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { useServer } from "@/features/services/hooks/use-server";
import { ServiceDiskSkeleton } from "@/common/components/route-skeletons";
import {
  DEFAULT_RANGE_PRESET,
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
} from "@/features/metrics/lib/range";

export const Route = createFileRoute("/services/$serviceId/disk")({
  head: serviceTabHead("services.navDisk", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceDiskSkeleton,
  // The disk chart's time range persists to the URL in the service Metrics
  // tab's shape (`range`, `rangeStart`/`rangeEnd` — w6/065, w4/703), so a
  // picked window survives a reload and can be linked.
  validateSearch: parseRangeSearch,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <DiskRoute serviceId={serviceId} />
    </NonStaticRoute>
  );
}

/** Unexported: a route module must not export its page component. */
function DiskRoute({ serviceId }: { serviceId: string }) {
  const { service } = useServer(serviceId);
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="p-4 sm:p-6">
      <DiskSection
        serviceId={serviceId}
        plan={service?.plan ?? null}
        serviceType={service?.type ?? null}
        range={rangeFromSearch(search, DEFAULT_RANGE_PRESET)}
        onRangeChange={(range) =>
          // Functional form: committed verbatim, so rangeToSearch's explicit
          // undefineds clear stale custom bounds (w7/m42).
          void navigate({ search: () => rangeToSearch(range), replace: true })
        }
      />
    </div>
  );
}
