import { createFileRoute } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { EmptyState } from "@/common/components/empty-state";

export const Route = createFileRoute("/services/$serviceId/logs")({
  component: ServiceLogsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Logs · bex dashboard` }],
  }),
});

// Placeholder until w5/m6 ships live log tailing over bex-api's Logs API
// (docs/observability.md). A labeled nav target, not a broken route.
export function ServiceLogsPage() {
  const { t } = useTranslations();
  return (
    <EmptyState
      iconName="ScrollText"
      title={t("services.logsComingSoonTitle")}
      description={t("services.logsComingSoonBody")}
    />
  );
}
