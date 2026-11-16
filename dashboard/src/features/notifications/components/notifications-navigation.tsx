import { BellRing, Mail, Smartphone } from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for `/notifications`: one entry per delivery
 * channel, in page order. The browser section renders nothing on a server
 * without web push, and SectionNavigation drops a link whose section is empty.
 */
export function NotificationsNavigation({ className }: { className?: string }) {
  const { t } = useTranslations();
  return (
    <SectionNavigation
      ariaLabel={t("notifications.sectionNavigation")}
      className={className}
      items={[
        { href: "#email", label: t("notifications.emailTitle"), icon: Mail },
        {
          href: "#mobile-push",
          label: t("notifications.pushTitle"),
          icon: Smartphone,
        },
        {
          href: "#browser",
          label: t("notifications.webPushTitle"),
          icon: BellRing,
        },
      ]}
    />
  );
}
