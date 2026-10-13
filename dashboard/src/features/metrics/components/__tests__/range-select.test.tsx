import { describe, it, expect, vi, beforeAll } from "vitest";
import { useState } from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RangeSelect } from "../range-select";
import {
  DEFAULT_RANGE_PRESET,
  type CustomRange,
  type RangeSelection,
} from "../../lib/range";

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

// A host that accepts Apply, like the URL-backed Logs/Metrics routes do.
function StatefulRangeSelect({
  initial,
  onRangeChange,
}: {
  initial: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
}) {
  const [range, setRange] = useState(initial);
  return (
    <RangeSelect
      range={range}
      onRangeChange={(next) => {
        onRangeChange(next);
        setRange(next);
      }}
    />
  );
}

const iso = (local: string) => new Date(local).toISOString();

describe("RangeSelect edit active custom range (w4/245)", () => {
  it("shows no edit action for a relative preset", () => {
    render(
      <RangeSelect range={DEFAULT_RANGE_PRESET} onRangeChange={vi.fn()} />,
    );
    expect(
      screen.queryByRole("button", { name: "Edit custom range" }),
    ).not.toBeInTheDocument();
  });

  it("reopens an applied custom window seeded with its bounds; Cancel keeps it, a second Apply edits it", async () => {
    const user = userEvent.setup();
    const onRangeChange = vi.fn();
    render(
      <StatefulRangeSelect
        initial={DEFAULT_RANGE_PRESET}
        onRangeChange={onRangeChange}
      />,
    );

    await user.click(screen.getByLabelText("Time range"));
    await user.click(screen.getByRole("option", { name: /Custom/ }));
    fireEvent.change(screen.getByLabelText("Start"), {
      target: { value: "2026-10-10T17:45" },
    });
    fireEvent.change(screen.getByLabelText("End"), {
      target: { value: "2026-10-10T17:46" },
    });
    await user.click(screen.getByRole("button", { name: "Apply" }));
    expect(onRangeChange).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Reopen without detouring through a preset: seeded to the applied minute.
    await user.click(screen.getByRole("button", { name: "Edit custom range" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Start")).toHaveValue("2026-10-10T17:45");
    expect(screen.getByLabelText("End")).toHaveValue("2026-10-10T17:46");

    // Cancel leaves the range untouched; reopening still shows the bounds.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onRangeChange).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Edit custom range" }));
    expect(screen.getByLabelText("End")).toHaveValue("2026-10-10T17:46");

    fireEvent.change(screen.getByLabelText("End"), {
      target: { value: "2026-10-10T17:47" },
    });
    await user.click(screen.getByRole("button", { name: "Apply" }));
    expect(onRangeChange).toHaveBeenCalledTimes(2);
    expect(onRangeChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        id: "custom",
        startTime: iso("2026-10-10T17:45"),
        endTime: iso("2026-10-10T17:47"),
      }),
    );
  });

  // w4/279: the footer Cancel bypassed the dialog's close handler, so the
  // rejected-Apply warning survived and reappeared beside the reseeded dates.
  it.each([
    ["2026-10-10T17:46", "The end must be after the start."],
    ["2026-09-01T17:45", "Choose a window of 30 days or less."],
  ])(
    "drops a rejected-Apply warning on Cancel (start %s)",
    async (badStart, warning) => {
      const user = userEvent.setup();
      const onRangeChange = vi.fn();
      render(
        <StatefulRangeSelect
          initial={{
            id: "custom",
            startTime: iso("2026-10-10T17:45"),
            endTime: iso("2026-10-10T17:46"),
            resolutionSeconds: 60,
          }}
          onRangeChange={onRangeChange}
        />,
      );

      await user.click(
        screen.getByRole("button", { name: "Edit custom range" }),
      );
      fireEvent.change(screen.getByLabelText("Start"), {
        target: { value: badStart },
      });
      await user.click(screen.getByRole("button", { name: "Apply" }));
      expect(screen.getByRole("alert")).toHaveTextContent(warning);

      await user.click(screen.getByRole("button", { name: "Cancel" }));
      await user.click(
        screen.getByRole("button", { name: "Edit custom range" }),
      );
      expect(screen.getByLabelText("Start")).toHaveValue("2026-10-10T17:45");
      expect(screen.getByLabelText("End")).toHaveValue("2026-10-10T17:46");
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      expect(onRangeChange).not.toHaveBeenCalled();

      fireEvent.change(screen.getByLabelText("End"), {
        target: { value: "2026-10-10T17:47" },
      });
      await user.click(screen.getByRole("button", { name: "Apply" }));
      expect(onRangeChange).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({
          startTime: iso("2026-10-10T17:45"),
          endTime: iso("2026-10-10T17:47"),
        }),
      );
    },
  );

  it("edits a custom range loaded from the URL with the keyboard", async () => {
    const user = userEvent.setup();
    const onRangeChange = vi.fn();
    render(
      <RangeSelect
        range={{
          id: "custom",
          startTime: iso("2026-10-10T17:45"),
          endTime: iso("2026-10-10T17:46"),
          resolutionSeconds: 60,
        }}
        onRangeChange={onRangeChange}
      />,
    );

    screen.getByRole("button", { name: "Edit custom range" }).focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Start")).toHaveValue("2026-10-10T17:45");
    expect(onRangeChange).not.toHaveBeenCalled();
  });
});
