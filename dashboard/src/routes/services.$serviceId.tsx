import { createFileRoute, Outlet, Link } from "@tanstack/react-router";
import { SearchX, TriangleAlert } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
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
  const { service, loading, error, refetch } = useServer(serviceId);
  const { pending, run } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();

  // A failed `server(id)` query is not evidence that the service is absent.
  // Keep it distinct from not-found so schema skew, auth failures, and backend
  // outages never masquerade as a deleted service.
  if (!service && !loading && error) {
    return (
      <DashboardLayout>
        <div className="flex-1 overflow-auto p-4 sm:p-6">
          <div className="mx-auto w-full max-w-4xl">
            <Card>
              <CardContent className="flex flex-col items-center justify-center gap-4 py-16 text-center">
                <TriangleAlert className="text-destructive h-8 w-8" />
                <div>
                  <p className="mb-1 font-medium">
                    {t("services.detailErrorTitle")}
                  </p>
                  <p className="text-muted-foreground text-sm">
                    {t("services.detailErrorBody")}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void refetch()}
                >
                  {t("common.tryAgain")}
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </DashboardLayout>
    );
  }

  // Unknown service id: `server(id)` resolved null and we're no longer loading.
  // Render a proper not-found state for the WHOLE detail (every tab) instead of
  // service chrome — never let a child tab borrow another service's data (the
  // 2026-07-09 phantom-service bug the fixed stub also guards against).
  if (!service && !loading) {
    return (
      <DashboardLayout>
        <div className="flex-1 overflow-auto p-4 sm:p-6">
          <div className="mx-auto w-full max-w-4xl">
            <Card>
              <CardContent className="flex flex-col items-center justify-center gap-4 py-16 text-center">
                <SearchX className="text-muted-foreground/50 h-8 w-8" />
                <div>
                  <p className="mb-1 font-medium">
                    {t("services.notFoundTitle")}
                  </p>
                  <p className="text-muted-foreground text-sm">
                    {t("services.notFoundBody", { name: serviceId })}
                  </p>
                </div>
                <Button asChild variant="outline" size="sm">
                  <Link to="/">{t("services.notFoundBackToList")}</Link>
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="min-h-0 flex-1 overflow-auto">
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
        <div className="p-4 sm:p-6">
          <div className="mx-auto w-full max-w-4xl space-y-6">
            <Outlet />
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
