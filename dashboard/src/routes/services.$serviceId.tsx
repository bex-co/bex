import { createFileRoute, Outlet, Link } from "@tanstack/react-router";
import { SearchX, TriangleAlert } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { useLatestDeploy } from "@/features/deploys/hooks/use-latest-deploy";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import {
  deriveServiceType,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
} from "@/features/services/components/service-detail-header";
import { ServerDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/services/$serviceId")({
  component: RouteComponent,
  // No-arg requireAuth: `next` is the requested href (the old form passed the
  // literal "$serviceId" pattern), so a login bounce returns to the actual
  // service URL — id- or name-shaped.
  beforeLoad: requireAuth(),
  loader: ({ context, params }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: ServerDocument,
          variables: { id: params.serviceId },
          fetchPolicy: "network-only",
          errorPolicy: "all",
        }),
      (data) =>
        data?.server &&
        (data.server.displayName?.trim() || data.server.name?.trim())
          ? data.server
          : null,
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (service) => [
        service.displayName?.trim() || service.name,
        translatedText(
          SERVICE_TYPE_LABEL[deriveServiceType(service.type ?? "")],
        ),
      ]),
      match,
    ),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceDetailLayout serviceId={serviceId} />;
}

/**
 * Shared chrome for every per-service page (Render's service-detail shape): the
 * overview header with lifecycle actions, the Overview/Logs nav, and an
 * `<Outlet/>` for the active tab. The parent route's network-only loader fills
 * Apollo's normalized cache for the title; this layout's cache-first
 * `useServer` read consumes that same result for the header without a second
 * title-only request.
 */
export function ServiceDetailLayout({ serviceId }: { serviceId: string }) {
  const { service, loading, error, refetch } = useServer(serviceId);
  const { deploy: latestDeploy } = useLatestDeploy(serviceId);
  const { pending } = useServiceLifecycle({ refetch });
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
            latestDeploy={latestDeploy}
            pending={pending?.id === service.id ? pending.action : null}
          />
        ) : (
          <ServiceDetailHeaderSkeleton name={serviceId} />
        )}
        <div className="p-4 sm:p-6">
          <div className="mx-auto w-full max-w-4xl space-y-6">
            <Outlet />
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}
