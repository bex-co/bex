import { createFileRoute, Outlet } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useServer } from "@/features/services/hooks/use-server";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
} from "@/features/services/components/service-detail-header";
import { ServiceNav } from "@/features/services/components/service-nav";

export const Route = createFileRoute("/services/$serviceId")({
  component: RouteComponent,
  beforeLoad: requireAuth("/services/$serviceId"),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceDetailLayout serviceId={serviceId} />;
}

/**
 * Shared chrome for every per-service page (Render's service-detail shape): the
 * overview header with lifecycle actions, the Overview/Logs nav, and an
 * `<Outlet/>` for the active tab. `server(id)` is read here for the header and
 * again in each child; Apollo's cache-and-network dedupes the two into one
 * request, so each route stays self-contained.
 */
export function ServiceDetailLayout({ serviceId }: { serviceId: string }) {
  const { service, refetch } = useServer(serviceId);
  const { pending, run } = useServiceLifecycle({ refetch });

  return (
    <DashboardLayout>
      {service ? (
        <ServiceDetailHeader
          service={service}
          pending={pending?.id === service.id ? pending.action : null}
          onRun={run}
        />
      ) : (
        <ServiceDetailHeaderSkeleton name={serviceId} />
      )}
      <ServiceNav serviceId={serviceId} />
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <Outlet />
        </div>
      </div>
    </DashboardLayout>
  );
}
