import {
  adaptMetricSeries,
  formatMetricValue,
  newestMetricTimestamp,
} from "../series";

describe("mobile metric snapshots", () => {
  it("sorts irregular points, totals aligned instances, and keeps resets", () => {
    const snapshot = adaptMetricSeries([
      {
        unit: "bytes",
        labels: [{ field: "instance", value: "a" }],
        values: [
          { time: "2026-01-01T00:02:00Z", value: 2 },
          { time: "2026-01-01T00:00:00Z", value: 8 },
          { time: "2026-01-01T00:01:00Z", value: 0 },
        ],
      },
      {
        unit: "bytes",
        labels: [{ field: "instance", value: "b" }],
        values: [{ time: "2026-01-01T00:02:00Z", value: 3 }],
      },
    ]);
    expect(snapshot.points.map((point) => point.value)).toEqual([8, 0, 5]);
    expect(snapshot.current).toBe(5);
    expect(snapshot.partial).toBe(false);
  });

  it("distinguishes empty and partial data while retaining valid samples", () => {
    expect(adaptMetricSeries([]).current).toBe(null);
    const partial = adaptMetricSeries([
      {
        unit: "cpu",
        labels: null,
        values: [
          { time: "bad", value: 4 },
          { time: "2026-01-01T00:00:00Z", value: 1 },
        ],
      },
    ]);
    expect(partial.points.length).toBe(1);
    expect(partial.partial).toBe(true);
  });

  it("carries degraded-source semantics and formats units", () => {
    const snapshot = adaptMetricSeries([
      {
        unit: "bytes",
        labels: [{ field: "degraded_sources", value: "nat,websocket" }],
        values: [{ time: "2026-01-01T00:00:00Z", value: 1024 }],
      },
    ]);
    expect(snapshot.degradedSources).toEqual(["nat", "websocket"]);
    expect(formatMetricValue(snapshot.unit, snapshot.current ?? 0)).toBe(
      "1.0 KiB",
    );
  });

  it("selects the newest real observation across sparse snapshots", () => {
    const older = adaptMetricSeries([
      {
        unit: "count",
        labels: null,
        values: [{ time: "2026-01-01T00:00:00Z", value: 1 }],
      },
    ]);
    const newer = adaptMetricSeries([
      {
        unit: "count",
        labels: null,
        values: [{ time: "2026-01-01T00:01:00Z", value: 2 }],
      },
    ]);
    expect(newestMetricTimestamp([older, newer])).toBe(
      "2026-01-01T00:01:00.000Z",
    );
    expect(newestMetricTimestamp([adaptMetricSeries([])])).toBe(null);
  });
});
