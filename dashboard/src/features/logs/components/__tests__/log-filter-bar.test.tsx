import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LogFilterBar } from "../log-filter-bar";
import {
  EMPTY_LOG_FILTERS,
  LOG_TYPE_REQUEST,
  type LogFilters,
} from "../../types";

// Radix Select uses the Pointer Capture API jsdom doesn't implement — same
// polyfill the registry-credential and create-wizard select tests carry.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

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

describe("LogFilterBar (w5/008, simplified w7/m42)", () => {
  it("keeps the primary toolbar minimal: type + search + range + live + Filters", () => {
    renderBar();
    expect(screen.getByLabelText("Log type")).toBeInTheDocument();
    expect(screen.getByLabelText("Search logs")).toBeInTheDocument();
    expect(screen.getByLabelText("Log history range")).toBeInTheDocument();
    expect(screen.getByRole("switch")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Filters" })).toBeInTheDocument();
    // The structured filters are behind the popover, not always visible.
    expect(screen.queryByLabelText("Level")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Method")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Status code")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Instance")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Request path")).not.toBeInTheDocument();
  });

  it("reveals the structured filter controls inside the Filters popover", async () => {
    const user = userEvent.setup();
    renderBar();
    await user.click(screen.getByRole("button", { name: "Filters" }));
    expect(screen.getByLabelText("Level")).toBeInTheDocument();
    expect(screen.getByLabelText("Method")).toBeInTheDocument();
    expect(screen.getByLabelText("Status code")).toBeInTheDocument();
    expect(screen.getByLabelText("Instance")).toBeInTheDocument();
    expect(screen.getByLabelText("Request path")).toBeInTheDocument();
  });

  it("emits a path patch as the user types in the popover", async () => {
    const user = userEvent.setup();
    const { onChange } = renderBar();
    await user.click(screen.getByRole("button", { name: "Filters" }));
    fireEvent.change(screen.getByLabelText("Request path"), {
      target: { value: "/api" },
    });
    expect(onChange).toHaveBeenCalledWith({ path: "/api" });
  });

  it("emits a level patch from the popover's dropdown", async () => {
    const user = userEvent.setup();
    const { onChange } = renderBar();
    await user.click(screen.getByRole("button", { name: "Filters" }));
    await user.click(screen.getByLabelText("Level"));
    await user.click(screen.getByRole("option", { name: "error" }));
    expect(onChange).toHaveBeenCalledWith({ level: "error" });
  });

  it("shows an active-filter count badge on the Filters trigger", () => {
    renderBar({ filters: { level: "error", path: "/api" } });
    const trigger = screen.getByRole("button", { name: /Filters/ });
    expect(trigger).toHaveTextContent("2");
  });

  it("renders active structured filters as chips and clears one via its ✕", () => {
    const { onChange } = renderBar({
      filters: { level: "error", statusCode: "5xx" },
    });
    expect(screen.getByText("Level: error")).toBeInTheDocument();
    expect(screen.getByText("Status code: 5xx")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Remove Level filter"));
    expect(onChange).toHaveBeenCalledWith({ level: "" });
    expect(onChange).not.toHaveBeenCalledWith({ statusCode: "" });
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
