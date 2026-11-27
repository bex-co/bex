import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentResourceSwitcher } from "../project-resource-filters";
import type { EnvironmentResourceFilter } from "@/features/projects/lib/resource-filter";

const counts = { all: 3, services: 2, envgroups: 1 };

function pressedNames() {
  const group = screen.getByRole("group", { name: "Resource type" });
  return within(group)
    .getAllByRole("button")
    .filter((button) => button.getAttribute("aria-pressed") === "true")
    .map((button) => button.textContent);
}

// Selection state must reach assistive technology, not just the styling.
describe("EnvironmentResourceSwitcher selection state (w4/333, w1/m682)", () => {
  it("offers Render's three categories and presses exactly the current one", async () => {
    const onChange = vi.fn();
    function Harness() {
      const [filter, setFilter] = useState<EnvironmentResourceFilter>("all");
      return (
        <EnvironmentResourceSwitcher
          filter={filter}
          counts={counts}
          onChange={(next) => {
            onChange(next);
            setFilter(next);
          }}
        />
      );
    }
    const user = userEvent.setup();
    render(<Harness />);
    const group = screen.getByRole("group", { name: "Resource type" });
    expect(
      within(group)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["All (3)", "Services (2)", "Env Groups (1)"]);
    for (const button of within(group).getAllByRole("button")) {
      expect(button).toHaveAttribute("aria-pressed");
    }
    expect(pressedNames()).toEqual(["All (3)"]);

    await user.click(screen.getByRole("button", { name: "Services (2)" }));
    expect(onChange).toHaveBeenLastCalledWith("services");
    expect(pressedNames()).toEqual(["Services (2)"]);

    screen.getByRole("button", { name: "Env Groups (1)" }).focus();
    await user.keyboard("[Enter]");
    expect(pressedNames()).toEqual(["Env Groups (1)"]);
  });

  it("shows an unread category's count as unknown, never zero (w4/m227)", () => {
    render(
      <EnvironmentResourceSwitcher
        filter="all"
        counts={{ all: null, services: null, envgroups: 0 }}
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "All (—)" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Services (—)" })).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Env Groups (0)" }),
    ).toBeVisible();
  });
});
