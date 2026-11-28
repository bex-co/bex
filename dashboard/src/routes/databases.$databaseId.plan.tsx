import { lazy, Suspense } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { DatabasePlanSkeleton } from "@/common/components/route-skeletons";
import { DatastorePlanPreview } from "@/common/components/datastore-plan-preview";
import { DatabaseHighAvailabilityPreview } from "@/common/components/datastore-overview-previews";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";

const DatabasePlanSection = lazy(() =>
  import("@/features/databases/components/database-plan-section").then((m) => ({
    default: m.DatabasePlanSection,
  })),
);
const HAPanel = lazy(() =>
  import("@/features/databases/components/ha-panel").then((m) => ({
    default: m.HAPanel,
  })),
);

/**
 * A Postgres database's Plan page, Render's `/d/<id>/plan` (w3/m641): the
 * compute plan with Storage and Storage Autoscaling under it, then High
 * Availability (the switch and failover; the read replicas are on Info, as on
 * Render, w3/116), in Render's order. Render's `/upgrade` and
 * `/high-availability-upgrade` land here (databases.$databaseId.$.tsx).
 */
export const Route = createFileRoute("/databases/$databaseId/plan")({
  head: databasePageHead("databases.navPlan"),
  component: RouteComponent,
  pendingComponent: DatabasePlanSkeleton,
});

function RouteComponent() {
  const { database, refetch } = useDatabaseDetail();
  return (
    <>
      <section id="plan" className="scroll-mt-6">
        <Suspense fallback={<DatastorePlanPreview kind="postgres" />}>
          <DatabasePlanSection
            database={database}
            onChanged={() => void refetch()}
          />
        </Suspense>
      </section>
      <section id="high-availability" className="scroll-mt-6">
        <Suspense fallback={<DatabaseHighAvailabilityPreview />}>
          <HAPanel database={database} refetch={refetch} />
        </Suspense>
      </section>
    </>
  );
}
