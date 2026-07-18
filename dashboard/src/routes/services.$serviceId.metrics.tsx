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

export const Route = createFileRoute("/services/$serviceId/metrics")({
  component: ServiceMetricsPage,
});

// The metrics tab. The service chrome (DashboardLayout + header + nav) and the
// content container come from the `services.$serviceId` layout route; this
// renders only the tab's own toolbar + charts into the shared `<Outlet/>`.
// Render's scoping (captured live 2026-07-17, w5/m42): the page level owns
// only time + the event-timeline controls; Percentage/Total, Status Code, and
// Percentile live on the cards they alter.
function ServiceMetricsPage() {
  const { serviceId } = Route.useParams();
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

      <ApplicationMetricsCard
        resource={serviceId}
        window={window}
        markers={markers}
      />
      <NetworkMetricsCard
        resource={serviceId}
        window={window}
        markers={markers}
      />
    </>
  );
}
