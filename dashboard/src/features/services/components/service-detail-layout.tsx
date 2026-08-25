import type { ReactNode } from "react";
import { Outlet } from "@tanstack/react-router";
import { TriangleAlert } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  resourceFailed,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { useLatestDeploy } from "@/features/deploys/hooks/use-latest-deploy";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import {
  ServiceBaseProvider,
  type ServiceBase,
} from "@/features/services/lib/service-base";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
} from "@/features/services/components/service-detail-header";
import { ServiceRouteContentSkeleton } from "@/common/components/route-skeletons";

/**
 * Shared chrome for every per-service page (Render's service-detail shape): the
 * overview header with lifecycle actions, the resource-scoped nav, and an
 * `<Outlet/>` for the active tab. Rendered by both service-detail route trees —
 * `/services/$serviceId` (compute types) and `/static/$serviceId` (static_site,
 * Render parity w5/m57). The parent route's network-only loader fills Apollo's
 * normalized cache for the title; this layout's cache-first `useServer` read
 * consumes that same result for the header without a second title-only request.
 */
export function ServiceDetailLayout({
  serviceId,
  base = "/services",
}: {
  serviceId: string;
  base?: ServiceBase;
}) {
  const { service, loading, error, refetch } = useServer(serviceId);
  const { deploy: latestDeploy } = useLatestDeploy(serviceId);
  const { pending } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();

  // Unknown service id: redirect home with a toast (w9/m55) — covering every
  // child tab at once. A genuine query error is excluded; it keeps the inline
  // retry state below.
  useNotFoundRedirect(resourceNotFound(service, loading, error));

  // `base` (the route tree this layout renders under) rides context so the
  // sidebar + every tab's intra-detail link stays within /services or /static
  // (Render parity, w5/m57) without re-sniffing the router.
  let content: ReactNode;
  // A failed `server(id)` query is not evidence that the service is absent.
  // Keep it distinct from not-found so schema skew, auth failures, and backend
  // outages never masquerade as a deleted service.
  if (resourceFailed(service, loading, error)) {
    content = (
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
  } else if (!service && !loading) {
    // Unknown service id: the redirect above is already in flight. Render only
    // skeleton chrome for the WHOLE detail (every tab) — never let a child tab
    // borrow another service's data (the 2026-07-09 phantom-service bug the
    // fixed stub also guards against), so the `<Outlet/>` stays unmounted.
    content = (
      <DashboardLayout>
        <div
          className="min-h-0 flex-1 overflow-auto"
          data-route-skeleton={
            !service
              ? base === "/static"
                ? "static-active-tab"
                : "service-active-tab"
              : undefined
          }
        >
          <ServiceDetailHeaderSkeleton />
        </div>
      </DashboardLayout>
    );
  } else {
    content = (
      <DashboardLayout>
        <div className="min-h-0 flex-1 overflow-auto">
          {service ? (
            <ServiceDetailHeader
              service={service}
              latestDeploy={latestDeploy}
              pending={pending?.id === service.id ? pending.action : null}
            />
          ) : (
            <ServiceDetailHeaderSkeleton />
          )}
          <div className="p-4 sm:p-6">
            {/* Detail tabs stay at the established 4xl width; the long Settings
                page opts into a 6xl shell so its right rail doesn't compress
                the existing form cards. */}
            <div
              data-skeleton-region={!service ? "active-tab" : undefined}
              className="mx-auto w-full max-w-4xl space-y-6 has-[>.service-settings-layout]:max-w-6xl"
            >
              {service ? (
                <Outlet />
              ) : (
                <ServiceRouteContentSkeleton base={base} />
              )}
            </div>
          </div>
        </div>
      </DashboardLayout>
    );
  }

  return <ServiceBaseProvider value={base}>{content}</ServiceBaseProvider>;
}
