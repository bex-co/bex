import { createFileRoute } from "@tanstack/react-router";
import { WebhookDeliveriesCard } from "@/features/webhooks/components/webhook-deliveries-card";
import { useWebhookDetail } from "@/features/webhooks/components/webhook-detail-context";

export const Route = createFileRoute("/webhook/$webhookId/")({
  component: WebhookActivityTab,
});

/**
 * Activity — the detail page's default view, Render's "Recent deliveries"
 * (w1/m49/t004). The shell already resolved the endpoint; this tab only adds
 * the deliveries table.
 */
function WebhookActivityTab() {
  const { endpoint } = useWebhookDetail();
  if (!endpoint) return null; // the shell renders loading/not-found states
  return <WebhookDeliveriesCard endpointId={endpoint.id} />;
}
