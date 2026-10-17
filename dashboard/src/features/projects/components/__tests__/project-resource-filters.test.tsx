import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectResourceFilters } from "../project-resource-filters";
import type { ProjectResourceFilterState } from "@/features/projects/lib/resource-filter";

const counts = {
  all: 2,
  services: 2,
  databases: 0,
  keyvalues: 0,
  envgroups: 0,
};

function pressedNames() {
  const group = screen.getByRole("group", { name: "Resource type" });
  return within(group)
    .getAllByRole("button")
    .filter((button) => button.getAttribute("aria-pressed") === "true")
    .map((button) => button.textContent);
}

// Selection state must reach assistive technology, not just the styling.
describe("ProjectResourceFilters selection state (w4/333)", () => {
  it("names the group and presses exactly the current kind", async () => {
    const onChange = vi.fn();
    function Harness() {
      const [filter, setFilter] = useState<ProjectResourceFilterState>({
        environmentId: "evm-1",
        query: "eden",
        kind: "all",
      });
      return (
        <ProjectResourceFilters
          environmentName="production"
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
    for (const button of within(group).getAllByRole("button")) {
      expect(button).toHaveAttribute("aria-pressed");
    }
    expect(pressedNames()).toEqual(["All (2)"]);

    await user.click(screen.getByRole("button", { name: "Services (2)" }));
    expect(onChange).toHaveBeenLastCalledWith({
      environmentId: "evm-1",
      query: "eden",
      kind: "services",
    });
    expect(pressedNames()).toEqual(["Services (2)"]);

    screen.getByRole("button", { name: "Databases (0)" }).focus();
    await user.keyboard("[Space]");
    expect(pressedNames()).toEqual(["Databases (0)"]);
    screen.getByRole("button", { name: "Env Groups (0)" }).focus();
    await user.keyboard("[Enter]");
    expect(pressedNames()).toEqual(["Env Groups (0)"]);
  });

  it("follows a filter supplied by the parent (URL kind)", () => {
    const filter = (kind: ProjectResourceFilterState["kind"]) => ({
      environmentId: null,
      query: "",
      kind,
    });
    const props = { environmentName: "Unassigned", counts, onChange: vi.fn() };
    const { rerender } = render(
      <ProjectResourceFilters {...props} filter={filter("databases")} />,
    );
    expect(pressedNames()).toEqual(["Databases (0)"]);
    rerender(
      <ProjectResourceFilters {...props} filter={filter("keyvalues")} />,
    );
    expect(pressedNames()).toEqual(["Key Values (0)"]);
  });
});
