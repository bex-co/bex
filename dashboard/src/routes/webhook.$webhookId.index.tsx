import { createFileRoute } from "@tanstack/react-router";
import { WebhookActivityTab } from "@/features/webhooks/components/webhook-activity-tab";
import { WebhookActivitySkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/webhook/$webhookId/")({
  component: WebhookActivityTab,
  pendingComponent: WebhookActivitySkeleton,
});
