import { lazy, Suspense } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { DatabaseRecoverySkeleton } from "@/common/components/route-skeletons";
import { RecoveryPanelSkeleton } from "@/features/databases/components/recovery-panel-skeleton";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";

const RecoveryPanel = lazy(() =>
  import("@/features/databases/components/recovery-panel").then((m) => ({
    default: m.RecoveryPanel,
  })),
);

/**
 * A Postgres database's Recovery page, Render's `/d/<id>/recovery`:
 * point-in-time recovery and exports (w3/m641).
 */
export const Route = createFileRoute("/databases/$databaseId/recovery")({
  head: databasePageHead("databases.navRecovery"),
  component: RouteComponent,
  pendingComponent: DatabaseRecoverySkeleton,
});

function RouteComponent() {
  const { database } = useDatabaseDetail();
  return (
    <section id="recovery" className="scroll-mt-6">
      <Suspense
        fallback={
          <RecoveryPanelSkeleton
            backupsEnabled={database.backupsEnabled !== false}
            exportScope={Boolean(database.databaseName)}
          />
        }
      >
        <RecoveryPanel
          id={database.id}
          databaseName={database.databaseName}
          backupsEnabled={database.backupsEnabled}
        />
      </Suspense>
    </section>
  );
}
