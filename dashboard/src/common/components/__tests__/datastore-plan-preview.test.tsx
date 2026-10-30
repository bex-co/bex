import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { render, within } from "@testing-library/react";
import { POSTGRES_PLAN_SLOTS } from "../datastore-plan-preview";
import {
  DatabaseOverviewSkeleton,
  KeyValueOverviewSkeleton,
} from "../route-skeletons";
import { CREATE_PLAN_CARD_GRID_CLASS } from "../plan-card-grid";

// w4/m208: a datastore plan region keeps the ready card's anatomy while its
// resource or catalog loads — title, description, one 70px tier slot per
// catalog tier in the ready grid (six Postgres plans since w2/m171, three Key
// Value), and the bordered Cancel/Save footer — and stays inert.
function planRegion(container: HTMLElement) {
  const region = container.querySelector('[data-skeleton-region="plan"]');
  expect(region).not.toBeNull();
  return region as HTMLElement;
}

// The slot counts are the catalogs' sizes, read from the shared tiers file so
// a tier added there without a slot here fails instead of a loading grid one
// row short of the ready one.
function catalogSize(family: string): number {
  const yaml = readFileSync(
    `${process.cwd()}/../lego/types/tiers/tiers.yaml`,
    "utf8",
  );
  const section = yaml
    .split(/^(?=\w+:\s*$)/m)
    .find((s) => s.startsWith(`${family}:`));
  expect(section, `tiers.yaml has no ${family} section`).toBeDefined();
  return [...section!.matchAll(/^\s+- id:/gm)].length;
}

describe("datastore plan slot counts", () => {
  it("match the Postgres and Key Value catalogs", () => {
    expect(POSTGRES_PLAN_SLOTS).toBe(catalogSize("postgres"));
    expect(catalogSize("valkey")).toBe(3);
  });
});

describe("datastore plan preview", () => {
  for (const [name, Skeleton, title, save, tiers] of [
    ["Postgres", DatabaseOverviewSkeleton, "Instance type", "Save", 6],
    ["Key Value", KeyValueOverviewSkeleton, "Instance type", "Save", 3],
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
      expect(slots).toHaveLength(tiers);
      for (const slot of slots) {
        expect(slot.className).toContain("p-3");
        expect(slot.className).toContain("border");
        const [nameLine, specsLine] = Array.from(slot.children);
        expect(nameLine.className).toContain("h-6"); // 24px name line
        expect(specsLine.className).toContain("h-5"); // 20px specs line
      }
      const buttons = within(region).getAllByRole("button", { hidden: true });
      expect(buttons.map((b) => b.textContent)).toEqual(["Cancel", save]);
      expect(buttons.every((b) => (b as HTMLButtonElement).disabled)).toBe(
        true,
      );
      expect(within(region).queryByRole("radio", { hidden: true })).toBeNull();
    });
  }
});
