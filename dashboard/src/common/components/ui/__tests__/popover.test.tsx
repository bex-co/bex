import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";

// w4/438: Radix flips a popover to the roomier side but cannot shrink it, so
// a tall panel (Logs → Filters, the Events filter) ran off a short window.
// jsdom does no layout; the guard is the cap and the scroll on the panel.
describe("PopoverContent", () => {
  it("caps its height at the space Radix reports and scrolls inside", () => {
    render(
      <Popover open>
        <PopoverTrigger>Open</PopoverTrigger>
        <PopoverContent className="w-80 p-0" aria-label="Details">
          body
        </PopoverContent>
      </Popover>,
    );
    const panel = screen
      .getByText("body")
      .closest('[data-slot="popover-content"]');
    expect(panel).toHaveClass(
      "max-h-(--radix-popover-content-available-height)",
      "overflow-y-auto",
      "w-80",
      "p-0",
    );
  });

  // w4/607: Radix renders the panel as a dialog and names it only from a
  // PopoverTitle, so a panel with neither was announced as just "dialog".
  it("is a dialog with the name it is given", () => {
    render(
      <Popover open>
        <PopoverTrigger>Open</PopoverTrigger>
        <PopoverContent aria-label="Log filters">body</PopoverContent>
      </Popover>,
    );
    expect(
      screen.getByRole("dialog", { name: "Log filters" }),
    ).toBeInTheDocument();
  });

  it("can be named by a heading it points at", () => {
    render(
      <Popover open>
        <PopoverTrigger>Open</PopoverTrigger>
        <PopoverContent aria-labelledby="panel-title">
          <h2 id="panel-title">Cost</h2>
        </PopoverContent>
      </Popover>,
    );
    expect(screen.getByRole("dialog", { name: "Cost" })).toBeInTheDocument();
  });

  it("does not compile without a name", () => {
    render(
      <Popover open>
        <PopoverTrigger>Open</PopoverTrigger>
        {/* @ts-expect-error a popover panel needs aria-label or aria-labelledby */}
        <PopoverContent>body</PopoverContent>
      </Popover>,
    );
  });
});
