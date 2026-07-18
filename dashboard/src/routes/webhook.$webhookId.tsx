import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { AlertTriangle } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { WebhookDetailContext } from "@/features/webhooks/components/webhook-detail-context";
import { WebhookDetailHeader } from "@/features/webhooks/components/webhook-detail-header";
import { useWebhook } from "@/features/webhooks/hooks/use-webhook";

export const Route = createFileRoute("/webhook/$webhookId")({
  component: WebhookDetailShell,
  beforeLoad: requireAuth(),
  head: ({ params }) => ({
    meta: [{ title: `${params.webhookId} · Webhook · bex dashboard` }],
  }),
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
  const { endpoint, loading, notFound } = detail;

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
      ) : loading ? (
        <div className="space-y-4 p-4 sm:p-6">
          <Skeleton className="h-24" />
          <Skeleton className="h-64" />
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 py-12 text-center">
          <AlertTriangle className="text-destructive size-8" />
          <p className="font-medium">
            {notFound ? t("webhooks.notFoundTitle") : t("webhooks.errorTitle")}
          </p>
          <p className="text-muted-foreground text-sm">
            {notFound
              ? t("webhooks.notFoundBody", { id: webhookId })
              : t("webhooks.errorBody")}
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
