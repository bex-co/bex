import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { WebhooksPanel } from "@/features/webhooks/components/webhooks-panel";

/**
 * Outbound event webhooks as a first-class page (w1/m45) — Render's sidebar
 * treats Webhooks as an Integrations page at `/webhooks` (live capture
 * 2026-07-16); the panel lived on account `/settings` since w3/m11. Same
 * placement-parity move m44 made for Team.
 */
export const Route = createFileRoute("/webhooks")({
  component: WebhooksPage,
  beforeLoad: requireAuth(),
  head: () => ({
    meta: [{ title: "Webhooks · bex dashboard" }],
  }),
});

function WebhooksPage() {
  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <WebhooksPanel />
        </div>
      </div>
    </DashboardLayout>
  );
}
