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
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";
import { NotificationsNavigation } from "@/features/notifications/components/notifications-navigation";
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
  const { t } = useTranslations();
  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={t("notifications.title")}
            description={t("notifications.pageDescription")}
          />
        }
        navigation={<NotificationsNavigation />}
      >
        <section id="email" className={SECTION_NAVIGATION_TARGET_CLASS}>
          <NotificationSettingsPanel />
        </section>
        <section id="mobile-push" className={SECTION_NAVIGATION_TARGET_CLASS}>
          <PushNotificationSettingsPanel />
        </section>
        <section id="browser" className={SECTION_NAVIGATION_TARGET_CLASS}>
          <WebPushSettingsPanel />
        </section>
      </PageFrame>
    </DashboardLayout>
  );
}
