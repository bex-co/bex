import { createFileRoute } from "@tanstack/react-router";
import { WebhookActivityTab } from "@/features/webhooks/components/webhook-activity-tab";
import { WebhookActivitySkeleton } from "@/common/components/route-skeletons";

/** Render's path for a webhook's Activity (w3/m707): its sidebar's Activity
 *  entry links `/webhook/<id>/activity`, and the bare URL opens it too. */
export const Route = createFileRoute("/webhook/$webhookId/activity")({
  component: WebhookActivityTab,
  pendingComponent: WebhookActivitySkeleton,
});
