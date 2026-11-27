import { WebhookDeliveriesCard } from "@/features/webhooks/components/webhook-deliveries-card";
import { useWebhookDetail } from "@/features/webhooks/components/webhook-detail-context";

/**
 * Activity — the detail page's default view, Render's "Recent deliveries"
 * (w1/m49/t004), at the bare URL and at Render's `/activity` (w3/m707). The
 * shell already resolved the endpoint; this tab only adds the deliveries
 * table.
 */
export function WebhookActivityTab() {
  const { endpoint } = useWebhookDetail();
  if (!endpoint) return null; // the shell renders loading/not-found states
  return (
    <WebhookDeliveriesCard
      endpointId={endpoint.id}
      endpointEnabled={endpoint.enabled}
    />
  );
}
