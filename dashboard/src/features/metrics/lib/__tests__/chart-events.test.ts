import { describe, it, expect } from "vitest";
import {
  toChartEventMarkers,
  clusterMarkers,
  type ChartEventMarker,
} from "../chart-events";
import type { ServiceEventView } from "@/features/events/hooks/use-service-events";

const WINDOW_START = "2026-07-17T10:00:00Z";
const WINDOW_END = "2026-07-17T11:00:00Z";

function event(overrides: Partial<ServiceEventView>): ServiceEventView {
  return {
    id: "evt-1",
    type: "deploy_started",
    timestamp: "2026-07-17T10:30:00Z",
    cursor: null,
    details: null,
    ...overrides,
  } as ServiceEventView;
}

describe("toChartEventMarkers", () => {
  it("keeps only events inside the window, sorted by time", () => {
    const markers = toChartEventMarkers(
      [
        event({ id: "evt-late", timestamp: "2026-07-17T10:45:00Z" }),
        event({ id: "evt-early", timestamp: "2026-07-17T10:15:00Z" }),
        event({ id: "evt-before", timestamp: "2026-07-17T09:59:00Z" }),
        event({ id: "evt-after", timestamp: "2026-07-17T11:01:00Z" }),
        event({ id: "evt-unstamped", timestamp: null }),
      ],
      WINDOW_START,
      WINDOW_END,
    );
    expect(markers.map((m) => m.id)).toEqual(["evt-early", "evt-late"]);
  });

  it("marks deploy_started with the neutral start treatment", () => {
    const [marker] = toChartEventMarkers(
      [event({ type: "deploy_started" })],
      WINDOW_START,
      WINDOW_END,
    );
    expect(marker.kind).toBe("start");
    expect(marker.labelKey).toBe("metrics.eventDeployStarted");
  });

  it("marks a succeeded deploy_ended green with the commit short id", () => {
    const [marker] = toChartEventMarkers(
      [
        event({
          type: "deploy_ended",
          details: {
            deployId: "dep-1",
            deployStatus: "succeeded",
            commitId: "a1318dbcafe0123",
          } as ServiceEventView["details"],
        }),
      ],
      WINDOW_START,
      WINDOW_END,
    );
    expect(marker.kind).toBe("success");
    expect(marker.labelKey).toBe("metrics.eventDeployLiveFor");
    expect(marker.labelParams).toEqual({ commit: "a1318db" });
    expect(marker.deployId).toBe("dep-1");
  });

  it("marks failed deploy_ended statuses as failures", () => {
    // "failed" is the wire enum (store.RenderDeployStatus); the raw internal
    // statuses are accepted defensively.
    for (const status of [
      "failed",
      "build_failed",
      "pre_deploy_failed",
      "update_failed",
    ]) {
      const [marker] = toChartEventMarkers(
        [
          event({
            type: "deploy_ended",
            details: {
              deployStatus: status,
            } as ServiceEventView["details"],
          }),
        ],
        WINDOW_START,
        WINDOW_END,
      );
      expect(marker.kind).toBe("failure");
      expect(marker.labelKey).toBe("metrics.eventDeployFailed");
    }
  });

  it("maps lifecycle events to the neutral info treatment", () => {
    const [marker] = toChartEventMarkers(
      [event({ type: "server_restarted" })],
      WINDOW_START,
      WINDOW_END,
    );
    expect(marker.kind).toBe("info");
    expect(marker.labelKey).toBe("metrics.eventServerRestarted");
  });

  it("falls back to a humanized type for unknown events", () => {
    const [marker] = toChartEventMarkers(
      [event({ type: "image_updated" })],
      WINDOW_START,
      WINDOW_END,
    );
    expect(marker.kind).toBe("info");
    expect(marker.labelKey).toBe("metrics.eventsChanged");
    expect(marker.fallbackLabel).toBe("image updated");
  });
});

describe("clusterMarkers", () => {
  const T0 = Date.parse("2026-07-17T10:00:00Z");
  // A 0..600 mapping over one hour: 1 minute = 10 viewBox units.
  const xForT = (t: number) => ((t - T0) / 3_600_000) * 600;

  function marker(id: string, minutes: number): ChartEventMarker {
    return {
      id,
      time: T0 + minutes * 60_000,
      kind: "info",
      labelKey: "metrics.eventsChanged",
    };
  }

  it("collapses markers within the overlap threshold into one cluster", () => {
    const clusters = clusterMarkers(
      [marker("a", 10), marker("b", 11), marker("c", 40)],
      xForT,
      0,
      600,
    );
    expect(clusters).toHaveLength(2);
    expect(clusters[0].markers.map((m) => m.id)).toEqual(["a", "b"]);
    // The cluster sits between its members.
    expect(clusters[0].x).toBeCloseTo(105);
    expect(clusters[1].markers.map((m) => m.id)).toEqual(["c"]);
  });

  it("keeps markers apart when they don't overlap", () => {
    const clusters = clusterMarkers(
      [marker("a", 10), marker("b", 20)],
      xForT,
      0,
      600,
    );
    expect(clusters).toHaveLength(2);
  });

  it("clamps out-of-domain markers to the plot edges", () => {
    const clusters = clusterMarkers(
      [marker("before", -30), marker("after", 90)],
      xForT,
      44,
      592,
    );
    expect(clusters.map((c) => c.x)).toEqual([44, 592]);
  });
});
