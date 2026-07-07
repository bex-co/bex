import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ApplicationMetricsCard } from "@/features/metrics/components/application-metrics-card";
import { NetworkMetricsCard } from "@/features/metrics/components/network-metrics-card";
import { MetricsFilters } from "@/features/metrics/components/metrics-filters";
import {
  DEFAULT_RANGE_PRESET,
  type RangePreset,
} from "@/features/metrics/lib/range";

export const Route = createFileRoute("/services/$serviceId/metrics")({
  component: ServiceMetricsPage,
  beforeLoad: requireAuth("/services/$serviceId/metrics"),
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Metrics · bex dashboard` }],
  }),
});

function ServiceMetricsPage() {
  const { serviceId } = Route.useParams();
  const [range, setRange] = useState<RangePreset>(DEFAULT_RANGE_PRESET);
  const [percentage, setPercentage] = useState(true); // Render defaults to Percentage
  const [quantile, setQuantile] = useState(0.95); // bex-api's own default quantile

  return (
    <DashboardLayout>
      <div className="flex-1 bg-background px-6 py-10 sm:px-10">
        <div className="mx-auto max-w-4xl space-y-6">
          <div>
            <Link
              to="/"
              className="text-sm text-muted-foreground hover:underline"
            >
              ← Services
            </Link>
            <h1 className="mt-1 text-2xl font-semibold text-foreground">
              {serviceId}
            </h1>
            <p className="text-sm text-muted-foreground">
              Metrics — live from bex-api
            </p>
          </div>

          <MetricsFilters
            range={range}
            onRangeChange={setRange}
            percentage={percentage}
            onPercentageChange={setPercentage}
            quantile={quantile}
            onQuantileChange={setQuantile}
          />

          <ApplicationMetricsCard
            resource={serviceId}
            percentage={percentage}
          />
          <NetworkMetricsCard
            resource={serviceId}
            range={range}
            quantile={quantile}
          />
        </div>
      </div>
    </DashboardLayout>
  );
}
