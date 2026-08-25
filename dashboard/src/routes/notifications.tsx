import { createFileRoute } from "@tanstack/react-router";
import { NotificationsPageSkeleton } from "@/common/components/route-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import {
  NotificationSettingsDocument,
  PushNotificationSettingsDocument,
} from "@/graphql/definitions";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { NotificationSettingsPanel } from "@/features/notifications/components/notification-settings-panel";
import { PushNotificationSettingsPanel } from "@/features/notifications/components/push-notification-settings-panel";
import { WebPushSettingsPanel } from "@/features/notifications/components/web-push-settings-panel";

/**
 * Notification settings as a first-class page (w1/m45) — Render's sidebar
 * treats Notifications as an Integrations page at `/notifications` (live
 * capture 2026-07-16); the panel lived on account `/settings` since w3/m9.
 */
export const Route = createFileRoute("/notifications")({
  staticData: { chrome: true },
  component: NotificationsPage,
  pendingComponent: NotificationsPageSkeleton,
  beforeLoad: requireAuth(),
  loader: ({ context, cause }) => {
    const fetchPolicy = titleLoaderFetchPolicy(cause);
    return prefetchInParallel([
      () =>
        context.client.query({
          query: NotificationSettingsDocument,
          fetchPolicy,
          errorPolicy: "all",
        }),
      () =>
        context.client.query({
          query: PushNotificationSettingsDocument,
          fetchPolicy,
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("notifications.title", match),
});

function NotificationsPage() {
  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <NotificationSettingsPanel />
          <PushNotificationSettingsPanel />
          <WebPushSettingsPanel />
        </div>
      </div>
    </DashboardLayout>
  );
}
