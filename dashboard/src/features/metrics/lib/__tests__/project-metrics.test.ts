import { describe, expect, it } from "vitest";
import {
  RESOURCE_DOT_CLASSES,
  RESOURCE_STROKE_COLORS,
} from "@/features/logs/lib/resource-labels";
import type { ChartSeries } from "@/features/metrics/types";
import {
  projectChartLines,
  projectChartNotes,
  projectLegend,
  type ProjectMetricResource,
} from "../project-metrics";

// A project's Service Metrics view (w3/m508): the legend, a chart's lines and
// the notes naming what a chart could not draw.

const resource = (
  id: string,
  name: string,
  status = "CHARTED",
  type = "web_service",
): ProjectMetricResource => ({ id, name, type, environmentId: null, status });

const series = (labels: Record<string, string>, value = 1): ChartSeries => ({
  unit: "cpu",
  labels,
  points: [{ timestamp: "2026-11-19T10:00:00Z", value }],
});

const RESOURCES = [
  resource("srv-web", "web"),
  resource("srv-api", "api"),
  resource("srv-docs", "docs", "NOT_CHARTED", "static_site"),
  resource("srv-jobs", "jobs", "FAILED", "background_worker"),
  resource("srv-cron", "nightly", "CHARTED", "cron_job"),
];

describe("projectLegend", () => {
  it("gives each resource the hue a project log view gives the same position", () => {
    const legend = projectLegend(RESOURCES);
    expect(legend.map((e) => e.name)).toEqual([
      "web",
      "api",
      "docs",
      "jobs",
      "nightly",
    ]);
    legend.forEach((entry, i) => {
      expect(entry.dotClass).toBe(RESOURCE_DOT_CLASSES[i]);
      expect(entry.color).toBe(RESOURCE_STROKE_COLORS[i]);
    });
    // The stroke is the dot's own color variable, so the two never drift.
    expect(RESOURCE_STROKE_COLORS[0]).toBe("var(--color-sky-500)");
  });

  it("cycles the hues past the palette and names a nameless resource by id", () => {
    const many = Array.from(
      { length: RESOURCE_DOT_CLASSES.length + 1 },
      (_, i) => resource(`srv-${i}`, i === 0 ? "" : `svc-${i}`),
    );
    const legend = projectLegend(many);
    expect(legend[0].name).toBe("srv-0");
    expect(legend.at(-1)?.color).toBe(legend[0].color);
  });
});

describe("projectChartLines", () => {
  const legend = projectLegend(RESOURCES);

  it("draws one named line per resource in its hue, none for a hidden one", () => {
    const lines = projectChartLines(
      [
        series({ RESOURCE: "srv-web" }),
        series({ RESOURCE: "srv-api" }),
        series({ RESOURCE: "srv-cron" }),
      ],
      legend,
      new Set(["srv-api"]),
      "var(--chart-1)",
    );
    expect(lines.map((l) => [l.label, l.color])).toEqual([
      ["web", RESOURCE_STROKE_COLORS[0]],
      ["nightly", RESOURCE_STROKE_COLORS[4]],
    ]);
  });

  it("names a status-code line by its code and an ungrouped line not at all", () => {
    const byCode = projectChartLines(
      [series({ STATUS_CODE: "200" }), series({ STATUS_CODE: "500" })],
      legend,
      new Set(),
      "var(--chart-1)",
    );
    expect(byCode.map((l) => l.label)).toEqual(["200", "500"]);
    expect(byCode[0].color).not.toBe(byCode[1].color);
    const total = projectChartLines(
      [series({})],
      legend,
      new Set(["srv-web"]),
      "var(--chart-1)",
    );
    expect(total).toEqual([
      { points: series({}).points, color: "var(--chart-1)" },
    ]);
  });
});

describe("projectChartNotes", () => {
  it("names a failed read, a type the chart does not draw and a resource with no data", () => {
    const notes = projectChartNotes(
      RESOURCES,
      [series({ RESOURCE: "srv-web" }), series({ RESOURCE: "srv-api" })],
      "RESOURCE",
      new Set(),
    );
    expect(notes).toEqual({
      failed: ["jobs"],
      notCharted: ["docs"],
      noData: ["nightly"],
    });
  });

  it("leaves out hidden resources, and no-data when the chart drew nothing or adds resources up", () => {
    expect(
      projectChartNotes(
        RESOURCES,
        [series({ RESOURCE: "srv-web" })],
        "RESOURCE",
        new Set(["srv-jobs", "srv-api", "srv-cron"]),
      ),
    ).toEqual({ failed: [], notCharted: ["docs"], noData: [] });
    // An empty chart already says it has no data.
    expect(
      projectChartNotes(RESOURCES, [], "RESOURCE", new Set()).noData,
    ).toEqual([]);
    // Added up, no line is a resource's own.
    expect(
      projectChartNotes(RESOURCES, [series({})], "NONE", new Set()),
    ).toEqual({ failed: ["jobs"], notCharted: ["docs"], noData: [] });
  });
});
