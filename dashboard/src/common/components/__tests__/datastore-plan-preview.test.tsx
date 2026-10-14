import { describe, expect, it } from "vitest";
import { render, within } from "@testing-library/react";
import {
  DatabaseOverviewSkeleton,
  KeyValueOverviewSkeleton,
} from "../route-skeletons";
import { CREATE_PLAN_CARD_GRID_CLASS } from "../plan-card-grid";

// w4/m208: a datastore plan region keeps the ready card's anatomy while its
// resource or catalog loads — title, description, three 70px tier slots in
// the ready grid, and the bordered Cancel/Save footer — and stays inert.
function planRegion(container: HTMLElement) {
  const region = container.querySelector('[data-skeleton-region="plan"]');
  expect(region).not.toBeNull();
  return region as HTMLElement;
}

describe("datastore plan preview", () => {
  for (const [name, Skeleton, title, save] of [
    ["Postgres", DatabaseOverviewSkeleton, "Instance type", "Save"],
    ["Key Value", KeyValueOverviewSkeleton, "Instance type", "Save"],
  ] as const) {
    it(`${name} detail skeleton previews the ready plan card`, () => {
      const { container } = render(<Skeleton />);
      const region = planRegion(container);
      const card = region.firstElementChild as HTMLElement;
      expect(card.hasAttribute("inert")).toBe(true);
      expect(within(region).getByText(title)).toBeTruthy();
      const grid = region.querySelector('[aria-hidden="true"]') as HTMLElement;
      expect(grid.className).toBe(CREATE_PLAN_CARD_GRID_CLASS);
      const slots = Array.from(grid.children) as HTMLElement[];
      expect(slots).toHaveLength(3);
      for (const slot of slots) {
        expect(slot.className).toContain("p-3");
        expect(slot.className).toContain("border");
        const [nameLine, specsLine] = Array.from(slot.children);
        expect(nameLine.className).toContain("h-6"); // 24px name line
        expect(specsLine.className).toContain("h-5"); // 20px specs line
      }
      const buttons = within(region).getAllByRole("button", { hidden: true });
      expect(buttons.map((b) => b.textContent)).toEqual(["Cancel", save]);
      expect(buttons.every((b) => (b as HTMLButtonElement).disabled)).toBe(true);
      expect(within(region).queryByRole("radio", { hidden: true })).toBeNull();
    });
  }
});
