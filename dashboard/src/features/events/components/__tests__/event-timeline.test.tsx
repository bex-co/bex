import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { EventTimeline } from "../event-timeline";
import {
  filterTimelineEvents,
  type EventTimelineFilter,
} from "../../lib/timeline";
import type { ServiceEventView } from "../../hooks/use-service-events";

const eventsState: {
  events: ServiceEventView[];
  loading: boolean;
  error: Error | undefined;
} = { events: [], loading: false, error: undefined };
const useEventsSpy = vi.fn();

vi.mock("../../hooks/use-service-events", () => ({
  useServiceEvents: (serviceId: string, options: Record<string, unknown>) => {
    useEventsSpy(serviceId, options);
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

describe("filterTimelineEvents lifecycle category", () => {
  const range = ["2026-10-10T11:00:00Z", "2026-10-10T12:00:00Z"] as const;
  const ids = (list: ServiceEventView[], filter: EventTimelineFilter) =>
    filterTimelineEvents(list, ...range, filter).map((item) => item.id);

  // The captured w4/236 dataset: runtime suspend/resume facts landed under
  // Configuration because Lifecycle only knew the suspender intents.
  const captured = [
    event("d1", "deploy_started", "2026-10-10T11:01:00Z"),
    event("d2", "deploy_ended", "2026-10-10T11:02:00Z"),
    event("n1", "display_name_changed", "2026-10-10T11:10:00Z"),
    event("n2", "display_name_changed", "2026-10-10T11:11:00Z"),
    event("idle", "idle_timeout_changed", "2026-10-10T11:12:00Z"),
    event("sa", "suspender_added", "2026-10-10T11:20:00Z"),
    event("ss", "service_suspended", "2026-10-10T11:20:05Z"),
    event("sr", "suspender_removed", "2026-10-10T11:25:00Z"),
    event("res", "service_resumed", "2026-10-10T11:25:05Z"),
  ];

  it("files runtime suspend/resume under Lifecycle, not Configuration", () => {
    expect(ids(captured, "all")).toHaveLength(9);
    expect(ids(captured, "deploy")).toEqual(["d1", "d2"]);
    expect(ids(captured, "lifecycle")).toEqual(["sa", "ss", "sr", "res"]);
    expect(ids(captured, "config")).toEqual(["n1", "n2", "idle"]);
  });

  it.each([
    "service_suspended",
    "service_resumed",
    "service_hibernated",
    "service_woken",
    "server_failed",
    "server_available",
    "server_restarted",
    "suspender_added",
    "suspender_removed",
    "instance_count_changed",
    "autoscaling_config_changed",
    "plan_changed",
  ])("%s is Lifecycle only", (type) => {
    const list = [event("x", type, "2026-10-10T11:30:00Z")];
    expect(ids(list, "lifecycle")).toEqual(["x"]);
    expect(ids(list, "config")).toEqual([]);
    expect(ids(list, "deploy")).toEqual([]);
  });

  it("keeps unknown and missing types under Configuration and drops bad timestamps", () => {
    const list = [
      event("unknown", "brand_new_fact", "2026-10-10T11:30:00Z"),
      { ...event("untyped", "", "2026-10-10T11:31:00Z"), type: null },
      event("start-edge", "service_woken", "2026-10-10T11:00:00Z"),
      event("end-edge", "service_hibernated", "2026-10-10T12:00:00Z"),
      event("late", "service_suspended", "2026-10-10T12:00:01Z"),
      event("invalid", "service_resumed", "not-a-date"),
    ] as ServiceEventView[];
    expect(ids(list, "config")).toEqual(["unknown", "untyped"]);
    expect(ids(list, "lifecycle")).toEqual(["start-edge", "end-edge"]);
    expect(ids(list, "all")).toEqual([
      "unknown",
      "untyped",
      "start-edge",
      "end-edge",
    ]);
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

    expect(useEventsSpy).toHaveBeenCalledWith("srv-web", {
      limit: 100,
      startTime: "2026-07-15T09:00:00Z",
      endTime: "2026-07-15T10:00:00Z",
      autoPaginate: true,
    });
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
