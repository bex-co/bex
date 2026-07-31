import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { AlertTriangle } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Skeleton } from "@/common/components/ui/skeleton";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { WebhookDetailContext } from "@/features/webhooks/components/webhook-detail-context";
import { WebhookDetailHeader } from "@/features/webhooks/components/webhook-detail-header";
import { useWebhook } from "@/features/webhooks/hooks/use-webhook";
import { WebhookEndpointDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/webhook/$webhookId")({
  component: WebhookDetailShell,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: WebhookDetailShell,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: WebhookEndpointDocument,
          variables: {
            id: params.webhookId,
            ownerId: context.workspaceId,
          },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => {
        const endpoint = data?.webhookEndpoint;
        return endpoint && (endpoint.name?.trim() || endpoint.url?.trim())
          ? endpoint
          : null;
      },
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (endpoint) => [
        endpoint.name?.trim() || endpoint.url,
        translatedText("webhooks.detailKicker"),
      ]),
      match,
    ),
});

/**
 * The per-webhook page shell at Render's exact path shape
 * (`/webhook/<whk-id>`, singular — docs/render-artifacts/webhooks-ui.md):
 * header parity block + Activity/Settings navigation over child routes. The
 * endpoint is fetched once here and shared with the tabs via context. Render
 * swaps in a per-webhook sidebar; bex uses in-page tabs like its other
 * detail pages (recorded divergence, w1/m49).
 */
function WebhookDetailShell() {
  const { webhookId } = Route.useParams();
  const { t } = useTranslations();
  const detail = useWebhook(webhookId);
  const { endpoint, loading, notFound, error } = detail;

  // `notFound` also settles true when the query itself failed (errorPolicy
  // "all" leaves data empty), so exclude `error`: a dead id redirects home
  // (w9/m55), a failed query stays put on the inline error state below. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(notFound && !error);
  useLoaderErrorRetry(Route.useLoaderData(), webhookId);

  return (
    <DashboardLayout>
      {endpoint ? (
        <>
          <WebhookDetailHeader endpoint={endpoint} />
          <nav className="flex gap-4 border-b px-4 sm:px-6">
            <TabLink to="/webhook/$webhookId" webhookId={webhookId} exact>
              {t("webhooks.tabActivity")}
            </TabLink>
            <TabLink to="/webhook/$webhookId/settings" webhookId={webhookId}>
              {t("webhooks.tabSettings")}
            </TabLink>
          </nav>
          <div className="flex-1 overflow-auto p-4 sm:p-6">
            <div className="mx-auto w-full max-w-4xl space-y-6">
              <WebhookDetailContext.Provider value={detail}>
                <Outlet />
              </WebhookDetailContext.Provider>
            </div>
          </div>
        </>
      ) : loading || (notFound && !error) ? (
        <div className="p-4 sm:p-6">
          <div className="mx-auto w-full max-w-4xl space-y-6">
            {/* endpoint header (url + meta) then the activity/settings card */}
            <div className="space-y-2">
              <Skeleton className="h-6 w-56" />
              <Skeleton className="h-4 w-72" />
            </div>
            <CardSkeleton rows={4} />
          </div>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 py-12 text-center">
          <AlertTriangle className="text-destructive size-8" />
          <p className="font-medium">{t("webhooks.errorTitle")}</p>
          <p className="text-muted-foreground text-sm">
            {t("webhooks.errorBody")}
          </p>
          <Link to="/webhooks" className="text-sm underline">
            {t("webhooks.backToList")}
          </Link>
        </div>
      )}
    </DashboardLayout>
  );
}

function TabLink({
  to,
  webhookId,
  exact,
  children,
}: {
  to: string;
  webhookId: string;
  exact?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      to={to}
      params={{ webhookId }}
      activeOptions={{ exact: exact ?? false }}
      className="text-muted-foreground -mb-px border-b-2 border-transparent px-1 py-2 text-sm font-medium"
      activeProps={{
        className: "border-primary text-foreground",
      }}
    >
      {children}
    </Link>
  );
}
