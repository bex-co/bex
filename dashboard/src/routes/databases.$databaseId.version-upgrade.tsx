import { createFileRoute } from "@tanstack/react-router";
import { DatabaseVersionUpgradeSkeleton } from "@/common/components/route-skeletons";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { DatabaseVersionUpgradePage } from "@/features/databases/pages/database-version-upgrade-page";

/**
 * A Postgres database's guided version-upgrade page, Render's
 * `/d/<id>/version-upgrade` (w3/m676). Like Render's, it has no sidebar
 * entry: Info's version row and the Info banners open it.
 */
export const Route = createFileRoute("/databases/$databaseId/version-upgrade")({
  head: databasePageHead("databases.versionUpgradeHeading"),
  component: DatabaseVersionUpgradePage,
  pendingComponent: DatabaseVersionUpgradeSkeleton,
});
