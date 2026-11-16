import { createFileRoute } from "@tanstack/react-router";
import { WebhooksListPageSkeleton } from "@/common/components/route-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { WebhookEndpointsDocument } from "@/graphql/definitions";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  WebhooksPageHeader,
  WebhooksPanel,
} from "@/features/webhooks/components/webhooks-panel";
import { PageFrame } from "@/common/components/page-frame";

/**
 * Outbound event webhooks as a first-class page (w1/m45) — Render's sidebar
 * treats Webhooks as an Integrations page at `/webhooks` (live capture
 * 2026-07-16); the panel lived on account `/settings` since w3/m11. Same
 * placement-parity move m44 made for Team.
 */
export const Route = createFileRoute("/webhooks")({
  staticData: { chrome: true },
  component: WebhooksPage,
  pendingComponent: WebhooksListPageSkeleton,
  beforeLoad: requireAuth(),
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    return prefetchInParallel([
      () =>
        context.client.query({
          query: WebhookEndpointsDocument,
          variables: { ownerId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("webhooks.title", match),
});

function WebhooksPage() {
  return (
    <DashboardLayout>
      <PageFrame header={<WebhooksPageHeader />}>
        <WebhooksPanel />
      </PageFrame>
    </DashboardLayout>
  );
}
