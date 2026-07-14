import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LogFilterBar } from "../log-filter-bar";
import {
  EMPTY_LOG_FILTERS,
  LOG_TYPE_REQUEST,
  type LogFilters,
} from "../../types";

// The dropdowns discover values over Apollo — stub the hook.
const discovered = vi.fn(() => [] as string[]);
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: (_resource: string, label: string) => discovered(label),
}));

function renderBar(over: Partial<Parameters<typeof LogFilterBar>[0]> = {}) {
  const onChange = vi.fn();
  const onLiveChange = vi.fn();
  const filters: LogFilters = { ...EMPTY_LOG_FILTERS, ...over.filters };
  render(
    <LogFilterBar
      resource="web"
      filters={filters}
      onChange={onChange}
      live={true}
      onLiveChange={onLiveChange}
      liveSupported={over.liveSupported ?? true}
    />,
  );
  return { onChange, onLiveChange };
}

beforeEach(() => {
  discovered.mockReturnValue([]);
});

describe("LogFilterBar (w5/008)", () => {
  it("renders the structured filter controls (level/method/status/instance/path)", () => {
    renderBar();
    expect(screen.getByLabelText("Level")).toBeInTheDocument();
    expect(screen.getByLabelText("Method")).toBeInTheDocument();
    expect(screen.getByLabelText("Status code")).toBeInTheDocument();
    expect(screen.getByLabelText("Instance")).toBeInTheDocument();
    expect(screen.getByLabelText("Request path")).toBeInTheDocument();
  });

  it("emits a path patch as the user types", () => {
    const { onChange } = renderBar();
    fireEvent.change(screen.getByLabelText("Request path"), {
      target: { value: "/api" },
    });
    expect(onChange).toHaveBeenCalledWith({ path: "/api" });
  });

  it("emits a text patch from the search box", () => {
    const { onChange } = renderBar();
    fireEvent.change(screen.getByLabelText("Search logs"), {
      target: { value: "boom" },
    });
    expect(onChange).toHaveBeenCalledWith({ text: "boom" });
  });

  it("disables the live toggle when a store-only filter is active", () => {
    renderBar({ filters: { type: LOG_TYPE_REQUEST }, liveSupported: false });
    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("keeps the live toggle enabled with no store-only filter", () => {
    renderBar({ liveSupported: true });
    expect(screen.getByRole("switch")).not.toBeDisabled();
  });
});
