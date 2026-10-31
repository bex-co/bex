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
        <PopoverContent className="w-80 p-0">body</PopoverContent>
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
});
