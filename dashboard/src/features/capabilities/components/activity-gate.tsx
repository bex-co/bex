import type { ReactNode } from "react";
import { EmptyState } from "@/common/components/empty-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

/**
 * Service activity — a service's events (a cron job's runs among them), its
 * deploys and every metric — is Render's "View service events" and "View
 * service metrics", which its role table gives every role but Billing
 * (w9/m621). bex-api refuses those reads to such a member (can_view_activity),
 * each refusal audited, so a page or panel of them renders through this gate:
 * once `denied("can_view_activity")` confirms the role, `fallback` (the role's
 * reason by default; `null` leaves a widget out) stands in and `children`
 * never mount, nor do their reads. While capabilities are still checking, the
 * children load as for everyone else, since bex-api refuses the reads anyway;
 * a re-check keeps a confirmed denial (CapabilitiesProvider), so a gated page
 * never flashes its content.
 */
export function ActivityGate({
  children,
  fallback = <ActivityDeniedState />,
}: {
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const denied = useCapabilities().denied("can_view_activity");
  return denied ? fallback : children;
}

/** The role's reason where a page or panel of service activity would be. */
export function ActivityDeniedState() {
  const { t } = useTranslations();
  return (
    <EmptyState
      iconName="LockKeyhole"
      title={t("capabilities.activityDeniedTitle")}
      description={t("capabilities.reasonCanViewActivity")}
    />
  );
}
