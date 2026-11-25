import { lazy, Suspense } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { DatabaseAppsSkeleton } from "@/common/components/route-skeletons";
import { AdminAppsPanelSkeleton } from "@/features/databases/components/admin-apps-panel-skeleton";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";

const AdminAppsPanel = lazy(() =>
  import("@/features/databases/components/admin-apps-panel").then((m) => ({
    default: m.AdminAppsPanel,
  })),
);

/** A Postgres database's Apps page, Render's `/d/<id>/apps`: pgAdmin and PgHero (w4/m551, w3/m641). */
export const Route = createFileRoute("/databases/$databaseId/apps")({
  head: databasePageHead("databases.navApps"),
  component: RouteComponent,
  pendingComponent: DatabaseAppsSkeleton,
});

function RouteComponent() {
  const { database } = useDatabaseDetail();
  return (
    <section id="apps" className="scroll-mt-6">
      <Suspense fallback={<AdminAppsPanelSkeleton />}>
        <AdminAppsPanel databaseId={database.id} />
      </Suspense>
    </section>
  );
}
