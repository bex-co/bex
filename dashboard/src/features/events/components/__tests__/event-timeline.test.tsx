import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { EventTimeline } from "../event-timeline";
import { filterTimelineEvents } from "../../lib/timeline";
import type { ServiceEventView } from "../../hooks/use-service-events";

const eventsState: {
  events: ServiceEventView[];
  loading: boolean;
  error: Error | undefined;
} = { events: [], loading: false, error: undefined };
const useEventsSpy = vi.fn();

vi.mock("../../hooks/use-service-events", () => ({
  useServiceEvents: (serviceId: string, limit: number) => {
    useEventsSpy(serviceId, limit);
    return { ...eventsState, refetch: vi.fn() };
  },
}));

function event(id: string, type: string, timestamp: string): ServiceEventView {
  return {
    id,
    type,
    timestamp,
    cursor: null,
    details: null,
  } as ServiceEventView;
}

beforeEach(() => {
  eventsState.events = [];
  eventsState.loading = false;
  eventsState.error = undefined;
  useEventsSpy.mockReset();
});

describe("filterTimelineEvents", () => {
  const events = [
    event("before", "deploy_started", "2026-07-15T08:59:00Z"),
    event("deploy", "deploy_ended", "2026-07-15T09:30:00Z"),
    event("life", "server_restarted", "2026-07-15T09:40:00Z"),
    event("config", "env_vars_changed", "2026-07-15T09:50:00Z"),
  ];

  it("composes the selected metrics range with the event category", () => {
    const args = [
      events,
      "2026-07-15T09:00:00Z",
      "2026-07-15T10:00:00Z",
    ] as const;
    expect(filterTimelineEvents(...args, "all").map((item) => item.id)).toEqual(
      ["deploy", "life", "config"],
    );
    expect(
      filterTimelineEvents(...args, "deploy").map((item) => item.id),
    ).toEqual(["deploy"]);
    expect(
      filterTimelineEvents(...args, "lifecycle").map((item) => item.id),
    ).toEqual(["life"]);
    expect(
      filterTimelineEvents(...args, "config").map((item) => item.id),
    ).toEqual(["config"]);
  });
});

describe("EventTimeline", () => {
  it("reuses the shared service-events read and shows a range-aware empty state", () => {
    render(
      <EventTimeline
        serviceId="srv-web"
        startTime="2026-07-15T09:00:00Z"
        endTime="2026-07-15T10:00:00Z"
      />,
    );

    expect(useEventsSpy).toHaveBeenCalledWith("srv-web", 100);
    expect(screen.getByRole("status")).toHaveTextContent("No events");
  });

  it("applies the selected metrics range to the loaded events", () => {
    eventsState.events = [
      event("deploy", "deploy_ended", "2026-07-15T09:30:00Z"),
      event("outside", "server_restarted", "2026-07-15T10:30:00Z"),
    ];
    render(
      <EventTimeline
        serviceId="srv-web"
        startTime="2026-07-15T09:00:00Z"
        endTime="2026-07-15T10:00:00Z"
      />,
    );

    expect(screen.getByText("deploy ended")).toBeInTheDocument();
    expect(screen.queryByText("server restarted")).not.toBeInTheDocument();
  });
});
