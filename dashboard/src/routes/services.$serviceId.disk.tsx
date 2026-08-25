import { createFileRoute } from "@tanstack/react-router";

import { DiskSection } from "@/features/services/components/disk-section";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { useServer } from "@/features/services/hooks/use-server";
import { ServiceDiskSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/disk")({
  component: RouteComponent,
  pendingComponent: ServiceDiskSkeleton,
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
  return (
    <div className="p-4 sm:p-6">
      <DiskSection
        serviceId={serviceId}
        plan={service?.plan ?? null}
        serviceType={service?.type ?? null}
      />
    </div>
  );
}
