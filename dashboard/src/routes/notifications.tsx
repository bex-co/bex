import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { NotificationSettingsPanel } from "@/features/notifications/components/notification-settings-panel";
import { PushNotificationSettingsPanel } from "@/features/notifications/components/push-notification-settings-panel";

/**
 * Notification settings as a first-class page (w1/m45) — Render's sidebar
 * treats Notifications as an Integrations page at `/notifications` (live
 * capture 2026-07-16); the panel lived on account `/settings` since w3/m9.
 */
export const Route = createFileRoute("/notifications")({
  component: NotificationsPage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("notifications.title", match),
});

function NotificationsPage() {
  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <NotificationSettingsPanel />
          <PushNotificationSettingsPanel />
        </div>
      </div>
    </DashboardLayout>
  );
}
