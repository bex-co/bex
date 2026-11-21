import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Render's logical-replication guidance on the Connections card
 * (render.com/docs/postgresql-logical-replication, w34/m541): off by default,
 * enabled on request for a Pro workspace and a database with at least 10 GB,
 * after which the user publishes with CREATE PUBLICATION … FOR TABLE and
 * subscribers use the external URL. Static copy, so the route-pending preview
 * renders the same block and the card keeps its height.
 */
export function LogicalReplicationNote() {
  const { t } = useTranslations();
  return (
    <div className="space-y-1 border-t pt-4">
      <p className="text-sm font-medium">
        {t("databases.connLogicalReplicationTitle")}
      </p>
      <p className="text-xs text-muted-foreground">
        {t("databases.connLogicalReplicationBody")}
      </p>
    </div>
  );
}
