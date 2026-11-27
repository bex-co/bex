import { Navigate } from "@tanstack/react-router";
import { DatabaseVersionUpgrade } from "@/features/databases/components/database-version-upgrade";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";
import { versionUpgradeEligible } from "@/features/databases/lib/version-upgrade";

/**
 * A Postgres database's guided version-upgrade page, Render's
 * `/d/<id>/version-upgrade` (w3/m676), which Info's version row opens. Render
 * offers it on a paid database only and draws nothing for a free one; bex
 * sends a free database's link on to Info, where its version is.
 */
export function DatabaseVersionUpgradePage() {
  const { database } = useDatabaseDetail();
  if (!versionUpgradeEligible(database)) {
    return (
      <Navigate
        to="/databases/$databaseId"
        params={{ databaseId: database.id }}
        replace
      />
    );
  }
  return <DatabaseVersionUpgrade database={database} />;
}
