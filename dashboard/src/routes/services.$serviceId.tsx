import { createFileRoute, Outlet } from "@tanstack/react-router";
import { TriangleAlert } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
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
  // A roll-window loader failure dehydrates `state: "error"` — re-run it once
  // (w1/m52) so the title/head recover with the data; the layout's Apollo
  // reads below heal on their own.
  useLoaderErrorRetry(Route.useLoaderData(), serviceId);
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

  // Unknown service id (`server(id)` resolved null, no error): redirect home
  // with a toast (w9/m55) — covering every child tab at once. Query errors are
  // excluded; they keep the inline retry state below.
  useNotFoundRedirect(!service && !loading && !error);

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

  // Unknown service id: the redirect above is already in flight. Render only
  // skeleton chrome for the WHOLE detail (every tab) — never let a child tab
  // borrow another service's data (the 2026-07-09 phantom-service bug the
  // fixed stub also guards against), so the `<Outlet/>` stays unmounted.
  if (!service && !loading) {
    return (
      <DashboardLayout>
        <div className="min-h-0 flex-1 overflow-auto">
          <ServiceDetailHeaderSkeleton name={serviceId} />
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
