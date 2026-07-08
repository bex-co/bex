import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SvgLineChart, EmptyChart, ChartLegend } from "../svg-line-chart";

const POINTS = [
  { timestamp: "2026-07-06T09:00:00Z", value: 1 },
  { timestamp: "2026-07-06T09:00:30Z", value: 2 },
  { timestamp: "2026-07-06T09:01:00Z", value: 1.5 },
];

describe("SvgLineChart", () => {
  it("renders a line for each point and labels the chart with the count", () => {
    render(
      <SvgLineChart
        unit="cpu"
        series={[{ points: POINTS, color: "var(--chart-1)" }]}
      />,
    );
    expect(
      screen.getByRole("img", { name: /3 data points/ }),
    ).toBeInTheDocument();
  });

  it("counts each distinct sample time once across multiple series", () => {
    render(
      <SvgLineChart
        unit="cpu"
        series={[
          { points: POINTS, color: "var(--chart-1)", label: "a" },
          { points: POINTS.slice(1), color: "var(--chart-2)", label: "b" },
        ]}
      />,
    );
    // 3 distinct times, even though series b contributes only 2 of them.
    expect(
      screen.getByRole("img", { name: /3 data points/ }),
    ).toBeInTheDocument();
  });

  it("renders the empty state instead of a chart when there are no points", () => {
    render(<SvgLineChart unit="cpu" series={[]} />);
    expect(screen.getByText("No data in range")).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});

describe("EmptyChart", () => {
  it("renders the 'no data' message at the given height", () => {
    const { container } = render(<EmptyChart height={120} />);
    expect(screen.getByText("No data in range")).toBeInTheDocument();
    expect(container.firstChild).toHaveStyle({ height: "120px" });
  });

  it("renders a custom message (the no-limit percentage state)", () => {
    render(<EmptyChart message="No limit configured" />);
    expect(screen.getByText("No limit configured")).toBeInTheDocument();
  });
});

describe("ChartLegend", () => {
  it("names each labeled series, hiding itself below two entries", () => {
    const { rerender } = render(
      <ChartLegend
        entries={[
          { color: "red", label: "web-a" },
          { color: "blue", label: "web-b" },
          { color: "green" }, // unlabeled — filtered out
        ]}
      />,
    );
    expect(screen.getByText("web-a")).toBeInTheDocument();
    expect(screen.getByText("web-b")).toBeInTheDocument();

    rerender(<ChartLegend entries={[{ color: "red", label: "solo" }]} />);
    expect(screen.queryByText("solo")).not.toBeInTheDocument();
  });
});
