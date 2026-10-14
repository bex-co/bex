import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MetricSection } from "../metric-section";
import { MetricError } from "../metric-error";
import { MetricUnavailable } from "../metric-unavailable";
import { EmptyChart, SvgLineChart } from "../svg-line-chart";
import { SvgBarChart } from "../svg-bar-chart";
import { axisLeftPad, CHART_PAD } from "../chart-layout";
import { DatastoreMetricsSkeleton } from "@/common/components/route-skeletons";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";

// w4/m209: the populated chart is a width-100% SVG scaled by its 600×180
// viewBox, so every non-data frame takes that ratio instead of a fixed 180px.
const RATIO = "600 / 180";

describe("chart frames share the populated chart's ratio", () => {
  it("loading, empty, error and unavailable frames are responsive", () => {
    const loading = {
      loading: true,
      series: [],
    } as unknown as UseMetricsResult;
    const { container } = render(
      <>
        <MetricSection title="Memory" result={loading}>
          <div />
        </MetricSection>
        <EmptyChart />
        <MetricError />
        <MetricUnavailable />
      </>,
    );
    const frames = [
      screen.getByRole("status", { name: "Loading…" }),
      screen.getByText("No data in range"),
      screen.getByRole("alert"),
      container.querySelector(".border-dashed.flex-col") as HTMLElement,
    ];
    for (const frame of frames) {
      expect(frame.style.aspectRatio).toBe(RATIO);
      expect(frame.style.height).toBe("");
    }
  });

  it("keeps an explicit caller height", () => {
    render(<MetricError height={120} />);
    expect(screen.getByRole("alert").style.height).toBe("120px");
  });
});

describe("Key Value metrics preview", () => {
  it("previews the real panel: headings, three ratio plots, no filter row", () => {
    const { container } = render(<DatastoreMetricsSkeleton kind="keyvalue" />);
    for (const heading of ["Metrics", "Disk", "Memory", "Connections"]) {
      expect(
        screen.getAllByText(new RegExp(`^${heading}`)).length,
      ).toBeGreaterThan(0);
    }
    const plots = Array.from(
      container.querySelectorAll<HTMLElement>(
        "[data-slot=skeleton], .animate-pulse",
      ),
    ).filter((el) => el.style.aspectRatio === RATIO);
    expect(plots).toHaveLength(3);
    expect(container.querySelector(".h-9")).toBeNull(); // no phantom filter controls
  });
});

describe("tick gutter", () => {
  it("is wide enough for a long byte label and never below the default", () => {
    expect(
      axisLeftPad(["0 B", "268.4 KiB", "536.7 KiB"]),
    ).toBeGreaterThanOrEqual(52);
    expect(axisLeftPad(["0", "1", "2"])).toBe(CHART_PAD.left);
  });

  it("feeds one gutter to the gridlines and tick labels of both charts", () => {
    const points = [
      { timestamp: "2026-10-14T00:00:00Z", value: 0 },
      { timestamp: "2026-10-14T00:01:00Z", value: 549_580 },
    ];
    const { container } = render(
      <>
        <SvgLineChart
          unit="bytes"
          series={[{ points, color: "var(--chart-1)" }]}
        />
        <SvgBarChart
          unit="bytes"
          series={[{ points, color: "var(--chart-1)" }]}
        />
      </>,
    );
    for (const svg of Array.from(container.querySelectorAll("svg"))) {
      const grid = svg.querySelector("line") as SVGLineElement;
      const label = svg.querySelector("text") as SVGTextElement;
      const left = Number(grid.getAttribute("x1"));
      expect(left).toBeGreaterThanOrEqual(52);
      expect(Number(label.getAttribute("x"))).toBe(left - 6);
    }
  });
});
