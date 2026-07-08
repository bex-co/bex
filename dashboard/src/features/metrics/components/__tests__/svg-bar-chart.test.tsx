import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SvgBarChart } from "../svg-bar-chart";

const POINTS = [
  { timestamp: "2026-07-06T09:00:00Z", value: 1 },
  { timestamp: "2026-07-06T09:00:30Z", value: 0 }, // a zero-value bar must not crash the geometry
  { timestamp: "2026-07-06T09:01:00Z", value: 2 },
];

function single(points = POINTS) {
  return [{ points, color: "var(--chart-4)" }];
}

describe("SvgBarChart", () => {
  it("renders one bar per point, each with its value/time in an aria-label", () => {
    render(<SvgBarChart unit="count" series={single()} />);
    expect(screen.getAllByRole("graphics-symbol")).toHaveLength(3);
    expect(screen.getByLabelText(/^2 at/)).toBeInTheDocument();
  });

  it("renders the empty state when there are no points", () => {
    render(<SvgBarChart unit="count" series={single([])} />);
    expect(screen.getByText("No data in range")).toBeInTheDocument();
    expect(screen.queryAllByRole("graphics-symbol")).toHaveLength(0);
  });

  it("does not throw when every value is zero (would divide by zero for bar height)", () => {
    const zeros = [
      { timestamp: "2026-07-06T09:00:00Z", value: 0 },
      { timestamp: "2026-07-06T09:00:30Z", value: 0 },
    ];
    expect(() =>
      render(<SvgBarChart unit="count" series={single(zeros)} />),
    ).not.toThrow();
  });

  it("stacks multiple series per time bucket and labels the bar with the stack total", () => {
    render(
      <SvgBarChart
        unit="count"
        series={[
          { points: [POINTS[0]], color: "red", label: "200" },
          { points: [{ ...POINTS[0], value: 2 }], color: "blue", label: "500" },
        ]}
      />,
    );
    // One frame (shared timestamp), aria-label carries the 1+2=3 stack total.
    expect(screen.getAllByRole("graphics-symbol")).toHaveLength(1);
    expect(screen.getByLabelText(/^3 at/)).toBeInTheDocument();
  });
});
