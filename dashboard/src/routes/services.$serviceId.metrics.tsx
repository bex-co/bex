import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { ApplicationMetricsCard } from "@/features/metrics/components/application-metrics-card";
import { NetworkMetricsCard } from "@/features/metrics/components/network-metrics-card";
import { MetricsFilters } from "@/features/metrics/components/metrics-filters";
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

  return (
    <>
      <MetricsFilters
        range={range}
        onRangeChange={setRange}
        percentage={percentage}
        onPercentageChange={setPercentage}
        quantile={quantile}
        onQuantileChange={setQuantile}
      />

      <ApplicationMetricsCard resource={serviceId} percentage={percentage} />
      <NetworkMetricsCard resource={serviceId} range={range} quantile={quantile} />
    </>
  );
}
