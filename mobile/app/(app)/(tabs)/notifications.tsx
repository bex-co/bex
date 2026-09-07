import { NotificationInboxScreen } from "@/features/notifications/notifications-screen";
import { LazyTabScreen } from "@/components/lazy-tab-screen";

export default function NotificationsScreen() {
  return (
    <LazyTabScreen>
      <NotificationInboxScreen />
    </LazyTabScreen>
  );
}
