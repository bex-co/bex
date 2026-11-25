import { lazy, Suspense } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  DatabaseMetricsSkeleton,
  DatastoreMetricsSkeleton,
} from "@/common/components/route-skeletons";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { DeferredMount } from "@/common/components/deferred-mount";
import {
  DEFAULT_RANGE_PRESET,
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { useDatabaseDetail } from "@/features/databases/lib/database-detail";
import type { DatabaseDetailView } from "@/features/databases/types";

const DatastoreMetricsPanel = lazy(() =>
  import("@/features/metrics/components/datastore-metrics-panel").then((m) => ({
    default: m.DatastoreMetricsPanel,
  })),
);
const InsightsPanel = lazy(() =>
  import("@/features/databases/components/insights-panel").then((m) => ({
    default: m.InsightsPanel,
  })),
);

/**
 * A Postgres database's Metrics page, Render's `/d/<id>/metrics` (w3/m641):
 * the charts, then Insights, as Render keeps its table and index sizes,
 * processes and top queries below its charts. The range persists in the
 * shared `range`/`rangeStart`/`rangeEnd` URL keys (w6/065, w4/703), so a
 * picked window survives a reload and a copied link.
 */
export const Route = createFileRoute("/databases/$databaseId/metrics")({
  head: databasePageHead("databases.navMetrics"),
  component: RouteComponent,
  pendingComponent: DatabaseMetricsSkeleton,
  validateSearch: parseRangeSearch,
});

function RouteComponent() {
  const { database } = useDatabaseDetail();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <>
      <section id="metrics" className="scroll-mt-6">
        <Suspense fallback={<DatastoreMetricsSkeleton kind="database" />}>
          <DatabaseMetrics
            database={database}
            range={rangeFromSearch(search, DEFAULT_RANGE_PRESET)}
            onRangeChange={(range) =>
              // Functional form, deliberately: its return is committed
              // verbatim, so rangeToSearch's explicit undefineds actually
              // clear stale custom bounds (w7/m42).
              void navigate({
                search: () => rangeToSearch(range),
                replace: true,
              })
            }
          />
        </Suspense>
      </section>
      <section id="insights" className="scroll-mt-6">
        <DeferredMount hashId="insights" minHeight={360}>
          <Suspense fallback={<CardSkeleton rows={4} />}>
            <InsightsPanel id={database.id} />
          </Suspense>
        </DeferredMount>
      </section>
    </>
  );
}

/**
 * Render's Active Connections chart reads the plan's connection limit beside
 * its title (w6/138), as the Key Value one does, from the same cache-first
 * catalog the Details card reads.
 */
function DatabaseMetrics({
  database,
  range,
  onRangeChange,
}: {
  database: DatabaseDetailView;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
}) {
  const { instanceTypes } = useDatabaseInstanceTypes();
  return (
    <DatastoreMetricsPanel
      kind="database"
      resource={database.id}
      range={range}
      onRangeChange={onRangeChange}
      highAvailabilityEnabled={database.highAvailabilityEnabled}
      hasReadReplicas={(database.readReplicas?.length ?? 0) > 0}
      connectionLimit={
        instanceTypes.find((it) => it.id === database.plan)?.maxConnections
      }
    />
  );
}
