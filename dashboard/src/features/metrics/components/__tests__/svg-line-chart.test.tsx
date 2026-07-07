import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SvgLineChart, EmptyChart } from "../svg-line-chart";

const POINTS = [
  { timestamp: "2026-07-06T09:00:00Z", value: 1 },
  { timestamp: "2026-07-06T09:00:30Z", value: 2 },
  { timestamp: "2026-07-06T09:01:00Z", value: 1.5 },
];

describe("SvgLineChart", () => {
  it("renders a line for each point and labels the chart with the count", () => {
    render(<SvgLineChart points={POINTS} unit="cpu" color="var(--chart-1)" />);
    expect(
      screen.getByRole("img", { name: /3 data points/ }),
    ).toBeInTheDocument();
  });

  it("renders the empty state instead of a chart when there are no points", () => {
    render(<SvgLineChart points={[]} unit="cpu" color="var(--chart-1)" />);
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
});
