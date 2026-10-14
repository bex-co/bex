import { describe, it, expect, afterEach } from "vitest";
import { act, render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { SvgLineChart, EmptyChart } from "../svg-line-chart";
import { SvgBarChart } from "../svg-bar-chart";

// w4/297: chart accessible names, the per-bar names, and the default empty
// copy follow the selected language through the real translation hook.

const ONE = [{ timestamp: "2026-10-14T07:00:00Z", value: 1 }];
const THREE = [
  { timestamp: "2026-10-14T07:00:00Z", value: 1 },
  { timestamp: "2026-10-14T07:00:30Z", value: 2 },
  { timestamp: "2026-10-14T07:01:00Z", value: 3 },
];
const series = (points: typeof ONE) => [{ points, color: "var(--chart-1)" }];

async function setLanguage(lng: string) {
  await act(async () => {
    await i18n.changeLanguage(lng);
  });
}

afterEach(async () => {
  await setLanguage("en");
});

describe("chart accessible names", () => {
  it("uses native English singular/plural forms with the frame count", () => {
    const { rerender } = render(
      <SvgLineChart unit="count" series={series(ONE)} />,
    );
    expect(
      screen.getByRole("img", { name: "Line chart with 1 data point" }),
    ).toBeInTheDocument();
    rerender(<SvgLineChart unit="count" series={series(THREE)} />);
    expect(
      screen.getByRole("img", { name: "Line chart with 3 data points" }),
    ).toBeInTheDocument();

    rerender(<SvgBarChart unit="count" series={series(ONE)} />);
    expect(
      screen.getByRole("img", { name: "Bar chart with 1 data point" }),
    ).toBeInTheDocument();
    rerender(<SvgBarChart unit="count" series={series(THREE)} />);
    expect(
      screen.getByRole("img", { name: "Bar chart with 3 data points" }),
    ).toBeInTheDocument();
  });

  it("renders Chinese names, including per-bar names, after a mounted switch", async () => {
    render(
      <>
        <SvgLineChart unit="count" series={series(THREE)} />
        <SvgBarChart unit="count" series={series(ONE)} />
      </>,
    );
    expect(
      screen.getByRole("img", { name: "Line chart with 3 data points" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/^1 at /)).toBeInTheDocument();

    await setLanguage("zh");

    expect(
      screen.getByRole("img", { name: "包含 3 个数据点的折线图" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "包含 1 个数据点的柱状图" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/：1$/)).toBeInTheDocument();
    expect(screen.queryByLabelText(/ at /)).not.toBeInTheDocument();
  });
});

describe("chart empty state", () => {
  it("localizes the default message and survives empty→populated transitions", async () => {
    await setLanguage("zh");
    const { rerender } = render(
      <SvgLineChart unit="count" series={series([])} />,
    );
    expect(screen.getByText("此时间范围内暂无数据")).toBeInTheDocument();

    rerender(<SvgLineChart unit="count" series={series(THREE)} />);
    expect(
      screen.getByRole("img", { name: "包含 3 个数据点的折线图" }),
    ).toBeInTheDocument();

    rerender(<SvgBarChart unit="count" series={series([])} />);
    expect(screen.getByText("此时间范围内暂无数据")).toBeInTheDocument();
    rerender(<SvgBarChart unit="count" series={series(ONE)} />);
    expect(
      screen.getByRole("img", { name: "包含 1 个数据点的柱状图" }),
    ).toBeInTheDocument();
  });

  it("keeps explicit messages, including an explicit empty string", async () => {
    await setLanguage("zh");
    const { container, rerender } = render(
      <EmptyChart message="No limit configured" />,
    );
    expect(screen.getByText("No limit configured")).toBeInTheDocument();

    rerender(<EmptyChart message="" />);
    expect(container.firstChild).toHaveTextContent(/^$/);
    expect(screen.queryByText("此时间范围内暂无数据")).not.toBeInTheDocument();
  });
});
