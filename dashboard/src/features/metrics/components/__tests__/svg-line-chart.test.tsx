import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SvgLineChart, EmptyChart, ChartLegend } from "../svg-line-chart";
import type { ChartInstanceActions } from "../chart-instance-menu";

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

describe("SvgLineChart instance actions (w4/717)", () => {
  const actions = (running: string[]): ChartInstanceActions => ({
    shell: (id) => ({
      href: running.includes(id)
        ? `/services/srv-a/shell?instance=${id}`
        : null,
    }),
    logsHref: (id) => `/services/srv-a/logs?instance=${id}`,
    isFiltered: () => false,
    onToggleFilter: vi.fn(),
    onCopyId: vi.fn(),
  });

  it("shows Render's hover hint, opens the action card on keyboard navigation, and closes on Escape", async () => {
    const user = userEvent.setup();
    render(
      <SvgLineChart
        unit="cpu"
        series={[
          { points: POINTS, color: "var(--chart-1)", label: "inst-up" },
          { points: POINTS, color: "var(--chart-2)", label: "inst-gone" },
        ]}
        instanceActions={actions(["inst-up"])}
      />,
    );

    const chart = screen.getByRole("img");
    fireEvent.keyDown(chart, { key: "ArrowRight" });
    expect(screen.getByText("View actions for inst-up")).toBeInTheDocument();

    // ArrowDown moves selection to the second (stopped) instance in the frame.
    fireEvent.keyDown(chart, { key: "ArrowDown" });
    expect(screen.getByText("View actions for inst-gone")).toBeInTheDocument();

    // Enter opens the floating card with SSH disabled for the stopped instance.
    fireEvent.keyDown(chart, { key: "Enter" });
    const sshButton = screen.getByRole("button", { name: "SSH into instance" });
    expect(sshButton).toBeDisabled();
    expect(sshButton).toHaveAccessibleDescription(
      "You can only SSH into running instances. This one is not available.",
    );

    await user.keyboard("{Escape}");
    expect(
      screen.queryByRole("button", { name: "SSH into instance" }),
    ).not.toBeInTheDocument();
  });

  // Render heads the card with the instance's slug; a label that starts with
  // the service id would truncate to what every instance shares.
  it("heads the action card with the instance's slug", () => {
    const id = "srv-d9dd16roviqs738quds0-klmnopqrstuv01234567";
    render(
      <SvgLineChart
        unit="cpu"
        series={[
          { points: POINTS, color: "var(--chart-1)", label: id },
          { points: POINTS, color: "var(--chart-2)", label: `${id}x` },
        ]}
        instanceActions={{
          ...actions([id]),
          formatLabel: (instance) => `${instance.slice(0, 30)}…`,
        }}
      />,
    );
    const chart = screen.getByRole("img");
    fireEvent.keyDown(chart, { key: "ArrowRight" });
    fireEvent.keyDown(chart, { key: "Enter" });
    expect(screen.getByTitle(id)).toHaveTextContent(/^klmno$/);
  });
});
