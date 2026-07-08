import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { ApplicationMetricsCard } from "@/features/metrics/components/application-metrics-card";
import { NetworkMetricsCard } from "@/features/metrics/components/network-metrics-card";
import { MetricsFilters } from "@/features/metrics/components/metrics-filters";
import { useMetricsFilterValues } from "@/features/metrics/hooks/use-metrics-filter-values";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import {
  DEFAULT_RANGE_PRESET,
  type RangePreset,
} from "@/features/metrics/lib/range";

export const Route = createFileRoute("/services/$serviceId/metrics")({
  component: ServiceMetricsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Metrics · bex dashboard` }],
  }),
});

// The metrics tab. The service chrome (DashboardLayout + header + nav) and the
// content container come from the `services.$serviceId` layout route; this
// renders only the tab's own filters + charts into the shared `<Outlet/>`.
function ServiceMetricsPage() {
  const { serviceId } = Route.useParams();
  const [range, setRange] = useState<RangePreset>(DEFAULT_RANGE_PRESET);
  const [percentage, setPercentage] = useState(true); // Render defaults to Percentage
  const [quantile, setQuantile] = useState(0.95); // bex-api's own default quantile
  const [statusCode, setStatusCode] = useState(""); // "" = all
  const discoveredStatusCodes = useMetricsFilterValues(
    serviceId,
    "STATUS_CODE",
  );
  // ONE live window for the whole page (a single tick timer; both cards'
  // x-axes stay in sync). pollIntervalMs: 0 — the window's own tick already
  // forces a refetch by changing startTime/endTime; Apollo's poll timer would
  // just be a second, redundant schedule.
  const window = { ...useLiveRange(range), pollIntervalMs: 0 };

  return (
    <>
      <MetricsFilters
        range={range}
        onRangeChange={setRange}
        percentage={percentage}
        onPercentageChange={setPercentage}
        quantile={quantile}
        onQuantileChange={setQuantile}
        statusCode={statusCode}
        onStatusCodeChange={setStatusCode}
        discoveredStatusCodes={discoveredStatusCodes}
      />

      <ApplicationMetricsCard
        resource={serviceId}
        percentage={percentage}
        window={window}
      />
      <NetworkMetricsCard
        resource={serviceId}
        window={window}
        quantile={quantile}
        statusCode={statusCode}
      />
    </>
  );
}
