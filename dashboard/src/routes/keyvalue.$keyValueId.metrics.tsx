import { createFileRoute } from "@tanstack/react-router";
import { DatastoreMetricsSkeleton } from "@/common/components/route-skeletons";
import {
  DEFAULT_RANGE_PRESET,
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
} from "@/features/metrics/lib/range";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { keyValuePageHead } from "@/features/keyvalue/lib/key-value-page-head";
import { useKeyValueDetail } from "@/features/keyvalue/lib/key-value-detail";

/**
 * A Key Value instance's Metrics page, Render's `/r/<id>/metrics` (w3/m649).
 * The range persists in the shared `range`/`rangeStart`/`rangeEnd` URL keys
 * (w6/065, w4/703), so a picked window survives a reload and a copied link.
 * Render's Active Connections chart reads the plan's connection limit beside
 * its title (w6/m459), from the same cache-first catalog the plan card reads.
 */
export const Route = createFileRoute("/keyvalue/$keyValueId/metrics")({
  head: keyValuePageHead("keyvalue.navMetrics"),
  component: RouteComponent,
  pendingComponent: KeyValueMetricsSkeleton,
  validateSearch: parseRangeSearch,
});

function KeyValueMetricsSkeleton() {
  return <DatastoreMetricsSkeleton kind="keyvalue" />;
}

function RouteComponent() {
  const { keyValue } = useKeyValueDetail();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { instanceTypes } = useKeyValueInstanceTypes();
  return (
    <DatastoreMetricsPanel
      kind="keyvalue"
      resource={keyValue.id}
      range={rangeFromSearch(search, DEFAULT_RANGE_PRESET)}
      onRangeChange={(range) =>
        // Functional form, deliberately: its return is committed verbatim, so
        // rangeToSearch's explicit undefineds actually clear stale custom
        // bounds (w7/m42).
        void navigate({ search: () => rangeToSearch(range), replace: true })
      }
      connectionLimit={
        instanceTypes.find((it) => it.id === keyValue.plan)?.connectionLimit
      }
    />
  );
}
