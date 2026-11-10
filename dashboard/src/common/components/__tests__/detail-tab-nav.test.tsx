import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DetailTabNav } from "../detail-tab-nav";

function tabs(active: string, onSelect = vi.fn()) {
  return ["overview", "metrics", "logs"].map((id) => ({
    id,
    label: id,
    active: id === active,
    onSelect: () => onSelect(id),
  }));
}

// w21/027: the current tab is exposed to assistive technology, exactly one.
describe("DetailTabNav", () => {
  it.each(["overview", "metrics", "logs"])(
    "marks only %s as current",
    (active) => {
      render(<DetailTabNav label="Details" tabs={tabs(active)} />);
      const current = screen
        .getAllByRole("button")
        .filter((b) => b.getAttribute("aria-current") === "page");
      expect(current.map((b) => b.textContent)).toEqual([active]);
    },
  );

  it("selects a tab on click", async () => {
    const onSelect = vi.fn();
    render(<DetailTabNav label="Details" tabs={tabs("overview", onSelect)} />);
    await userEvent.click(screen.getByRole("button", { name: "logs" }));
    expect(onSelect).toHaveBeenCalledWith("logs");
    expect(screen.getByRole("navigation", { name: "Details" })).toBeVisible();
  });
});
