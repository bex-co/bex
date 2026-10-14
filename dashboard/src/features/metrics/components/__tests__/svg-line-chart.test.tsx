import { describe, it, expect } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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

// w4/296: the captured lightly-used CPU Percentage samples.
describe("SvgLineChart small positive values", () => {
  const SMALL = [
    { timestamp: "2026-10-14T06:40:00Z", value: 0.07821397872822744 },
    { timestamp: "2026-10-14T06:40:30Z", value: 0.2505198684260272 },
  ];

  it("labels distinct axis ticks and a positive ArrowRight tooltip", () => {
    const { container } = render(
      <SvgLineChart
        unit="percentage"
        series={[{ points: SMALL, color: "var(--chart-1)" }]}
      />,
    );
    const ticks = Array.from(container.querySelectorAll("svg text")).map(
      (t) => t.textContent,
    );
    expect(ticks).toEqual(["0%", "0.125%", "0.251%"]);

    fireEvent.keyDown(screen.getByRole("img"), { key: "ArrowRight" });
    expect(screen.getByText("0.0782%")).toBeInTheDocument();
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
