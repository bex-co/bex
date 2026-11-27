import { PageScroll } from "@/common/components/page-frame";
import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { WebhookDetailShellSkeleton } from "@/common/components/route-skeletons";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  ResourceStatePage,
  ResourceStateView,
} from "@/common/components/resource-not-found";
import { resourceReadState } from "@/common/lib/resource-read-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { WebhookDetailContext } from "@/features/webhooks/components/webhook-detail-context";
import { WebhookDetailHeader } from "@/features/webhooks/components/webhook-detail-header";
import { useWebhook } from "@/features/webhooks/hooks/use-webhook";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { WebhookEndpointDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/webhook/$webhookId")({
  staticData: { chrome: true },
  component: WebhookDetailShell,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: WebhookDetailShell,
  pendingMs: 0,
  // The pending frame is (or renders) the page itself: keep it in the page's
  // chunk instead of the default reference-file placement (vite.config.ts).
  codeSplitGroupings: [
    ["component", "pendingComponent"],
    ["errorComponent"],
    ["notFoundComponent"],
  ],
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: WebhookEndpointDocument,
          // By id alone, as useWebhook reads it (w4/443).
          variables: { id: params.webhookId },
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
  const { loading } = detail;
  // Judge this page's controls in the endpoint's own workspace (w4/641, as
  // w6/091 for services): until it is the selection, the shell skeleton stays
  // up so no admin-only control or secret paints for the wrong workspace.
  const adopting = useAdoptResourceWorkspace(
    webhookId,
    detail.endpoint?.ownerId,
  );
  const endpoint = adopting ? null : detail.endpoint;
  const creatorIdentity = useWorkspaceSubjectLabel(endpoint?.createdBy ?? "");

  // A dead id keeps its URL and draws Render's Page not found, and a refused
  // read Access denied, both with Go to Webhooks as Render's webhook page
  // has; any other failure draws "An error occurred" (w3/m706). A not-found
  // is REPORTED as an error by bex-api, so neither `endpoint === null` nor
  // `!error` decides it alone. A roll-window loader failure re-runs once
  // (w1/m52) so the title recovers.
  const loaderData = Route.useLoaderData();
  const readState = adopting
    ? null
    : resourceReadState(
        detail.endpoint,
        loading,
        detail.error,
        loaderData?.state,
      );
  useLoaderErrorRetry(loaderData, webhookId);

  if (readState) {
    return (
      <ResourceStatePage>
        <ResourceStateView
          state={readState}
          id={webhookId}
          action={{ label: t("common.goToWebhooks"), to: "/webhooks" }}
        />
      </ResourceStatePage>
    );
  }

  return (
    <DashboardLayout>
      {endpoint ? (
        <>
          <WebhookDetailHeader
            endpoint={endpoint}
            creatorIdentity={creatorIdentity}
          />
          <nav className="flex gap-4 border-b px-4 sm:px-6">
            <TabLink to="/webhook/$webhookId" webhookId={webhookId} exact>
              {t("webhooks.tabActivity")}
            </TabLink>
            <TabLink to="/webhook/$webhookId/settings" webhookId={webhookId}>
              {t("webhooks.tabSettings")}
            </TabLink>
          </nav>
          <PageScroll>
            <div className="mx-auto w-full max-w-5xl space-y-6">
              <WebhookDetailContext.Provider value={detail}>
                <Outlet />
              </WebhookDetailContext.Provider>
            </div>
          </PageScroll>
        </>
      ) : (
        <WebhookDetailShellSkeleton />
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
