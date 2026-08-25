import { createFileRoute } from "@tanstack/react-router";
import { WebhookSettingsCard } from "@/features/webhooks/components/webhook-settings-card";
import { useWebhookDetail } from "@/features/webhooks/components/webhook-detail-context";
import { WebhookSettingsSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/webhook/$webhookId/settings")({
  component: WebhookSettingsTab,
  pendingComponent: WebhookSettingsSkeleton,
});

/**
 * Settings — edit name/URL/events/status + type-to-confirm delete
 * (w1/m49/t005). Keyed by endpoint id so the form state reseeds if the
 * route's endpoint ever swaps under it.
 */
function WebhookSettingsTab() {
  const { endpoint } = useWebhookDetail();
  if (!endpoint) return null; // the shell renders loading/not-found states
  return <WebhookSettingsCard key={endpoint.id} endpoint={endpoint} />;
}
