import type { ReactNode } from "react";
import { EmptyState } from "@/common/components/empty-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

/**
 * Logs — a service's, a datastore's, a deploy's and a project view's — are
 * Render's "View service logs", which its role table gives Admin, Developer
 * and Contributor and denies Viewer and Billing (w9/145). bex-api refuses
 * those reads to such a member (can_view_logs), so a log page or panel
 * renders through this gate, as service activity renders through
 * ActivityGate: once `denied("can_view_logs")` confirms the role, `fallback`
 * (the role's reason by default) stands in and `children` never mount, nor
 * do their history reads and live tails. While capabilities are still
 * checking, the children load as for everyone else, and a re-check keeps a
 * confirmed denial (CapabilitiesProvider).
 */
export function LogsGate({
  children,
  fallback = <LogsDeniedState />,
}: {
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const denied = useCapabilities().denied("can_view_logs");
  return denied ? fallback : children;
}

/** The role's reason where a page or panel of logs would be. */
export function LogsDeniedState() {
  const { t } = useTranslations();
  return (
    <EmptyState
      iconName="LockKeyhole"
      title={t("capabilities.roleDeniedTitle")}
      description={t("capabilities.reasonCanViewLogs")}
    />
  );
}
