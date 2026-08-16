import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RangeSelect } from "../range-select";
import { DEFAULT_RANGE_PRESET, type CustomRange } from "../../lib/range";

// Radix Select/Dialog lean on APIs jsdom doesn't implement.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

describe("RangeSelect custom range (w5/m56)", () => {
  it("offers the Last 30 days preset and applies it", async () => {
    const user = userEvent.setup();
    const onRangeChange = vi.fn();
    render(
      <RangeSelect
        range={DEFAULT_RANGE_PRESET}
        onRangeChange={onRangeChange}
      />,
    );

    await user.click(screen.getByLabelText("Time range"));
    await user.click(screen.getByRole("option", { name: "Last 30 days" }));

    expect(onRangeChange).toHaveBeenCalledWith(
      expect.objectContaining({ id: "30d" }),
    );
  });

  it("opens the picker on Custom and applies an absolute start/end window", async () => {
    const user = userEvent.setup();
    const onRangeChange = vi.fn();
    render(
      <RangeSelect
        range={DEFAULT_RANGE_PRESET}
        onRangeChange={onRangeChange}
      />,
    );

    await user.click(screen.getByLabelText("Time range"));
    await user.click(screen.getByRole("option", { name: /Custom/ }));

    // The picker dialog opened rather than immediately applying a range.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onRangeChange).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Start"), {
      target: { value: "2026-07-01T00:00" },
    });
    fireEvent.change(screen.getByLabelText("End"), {
      target: { value: "2026-07-01T06:00" },
    });
    await user.click(screen.getByRole("button", { name: "Apply" }));

    expect(onRangeChange).toHaveBeenCalledWith(
      expect.objectContaining({ id: "custom" }),
    );
    const applied = onRangeChange.mock.calls.at(-1)?.[0] as CustomRange;
    expect(new Date(applied.endTime).getTime()).toBeGreaterThan(
      new Date(applied.startTime).getTime(),
    );
  });

  it("rejects an end-before-start window with an inline error and no apply", async () => {
    const user = userEvent.setup();
    const onRangeChange = vi.fn();
    render(
      <RangeSelect
        range={DEFAULT_RANGE_PRESET}
        onRangeChange={onRangeChange}
      />,
    );

    await user.click(screen.getByLabelText("Time range"));
    await user.click(screen.getByRole("option", { name: /Custom/ }));

    fireEvent.change(screen.getByLabelText("Start"), {
      target: { value: "2026-07-02T00:00" },
    });
    fireEvent.change(screen.getByLabelText("End"), {
      target: { value: "2026-07-01T00:00" },
    });
    await user.click(screen.getByRole("button", { name: "Apply" }));

    expect(
      screen.getByText("The end must be after the start."),
    ).toBeInTheDocument();
    expect(onRangeChange).not.toHaveBeenCalled();
  });
});
