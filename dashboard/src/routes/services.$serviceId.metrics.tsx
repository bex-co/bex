import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { ApplicationMetricsCard } from "@/features/metrics/components/application-metrics-card";
import { NetworkMetricsCard } from "@/features/metrics/components/network-metrics-card";
import { MetricsFilters } from "@/features/metrics/components/metrics-filters";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import {
  DEFAULT_RANGE_PRESET,
  type RangePreset,
} from "@/features/metrics/lib/range";
import { toChartEventMarkers } from "@/features/metrics/lib/chart-events";
import { EventTimeline } from "@/features/events/components/event-timeline";
import type { EventTimelineFilter } from "@/features/events/lib/timeline";
import { useServiceEvents } from "@/features/events/hooks/use-service-events";
import { useServer } from "@/features/services/hooks/use-server";
import { isStaticSite } from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/metrics")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceMetricsPage serviceId={serviceId} />;
}

// The metrics tab. The service chrome (DashboardLayout + header + nav) and the
// content container come from the `services.$serviceId` layout route; this
// renders only the tab's own toolbar + charts into the shared `<Outlet/>`.
// Render's scoping (captured live 2026-07-17, w5/m42): the page level owns
// only time + the event-timeline controls; Percentage/Total, Status Code, and
// Percentile live on the cards they alter. Exported taking `serviceId` as a
// prop (the ServiceScalingPage pattern) so a routing test can mount it without
// the file Route's param context.
export function ServiceMetricsPage({ serviceId }: { serviceId: string }) {
  const [range, setRange] = useState<RangePreset>(DEFAULT_RANGE_PRESET);
  // Render hides the timeline until its toolbar toggle reveals it.
  const [timelineShown, setTimelineShown] = useState(false);
  const [eventFilter, setEventFilter] = useState<EventTimelineFilter>("all");
  // ONE live window for the whole page (a single tick timer; both cards'
  // x-axes stay in sync). pollIntervalMs: 0 — the window's own tick already
  // forces a refetch by changing startTime/endTime; Apollo's poll timer would
  // just be a second, redundant schedule.
  const window = { ...useLiveRange(range), pollIntervalMs: 0 };

  // Chart event markers (Render parity): the same event feed the timeline
  // shows, mapped onto every chart as a vertical line + icon badge. Apollo
  // dedupes this against EventTimeline's identical query — one request.
  const { events } = useServiceEvents(serviceId, 100);
  const markers = useMemo(
    () => toChartEventMarkers(events, window.startTime, window.endTime),
    [events, window.startTime, window.endTime],
  );

  // A static_site has no pods — it serves from the object store via the shared
  // static-server (docs/ADR029-static-sites.md) — so the Application card's
  // CPU/memory/instance-count charts would render empty forever. Render's
  // static Metrics page is request/bandwidth-oriented; keep only the Network
  // card (its Traefik series attribute per-App: the operator names the static
  // site's Ingress after the App, w5/m48/t006). Gated on the loaded service so
  // a static site never fires the pod-metrics queries.
  const { service } = useServer(serviceId);
  const showApplicationCard = service != null && !isStaticSite(service);

  return (
    <>
      <MetricsFilters
        range={range}
        onRangeChange={setRange}
        eventFilter={eventFilter}
        onEventFilterChange={setEventFilter}
        timelineShown={timelineShown}
        onTimelineShownChange={setTimelineShown}
      />

      {timelineShown && (
        <EventTimeline
          serviceId={serviceId}
          startTime={window.startTime}
          endTime={window.endTime}
          filter={eventFilter}
        />
      )}

      {showApplicationCard && (
        <ApplicationMetricsCard
          resource={serviceId}
          window={window}
          markers={markers}
        />
      )}
      <NetworkMetricsCard
        resource={serviceId}
        window={window}
        markers={markers}
      />
    </>
  );
}
