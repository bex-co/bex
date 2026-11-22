import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { render, within } from "@testing-library/react";
import {
  KEY_VALUE_PLAN_SLOTS,
  POSTGRES_PLAN_SLOTS,
} from "../datastore-plan-preview";
import {
  DatabaseOverviewSkeleton,
  KeyValueOverviewSkeleton,
} from "../route-skeletons";
import {
  COMPUTE_PLAN_LIST_CLASS,
  COMPUTE_PLAN_ROW_CLASS,
} from "../compute-plan-list";

// w4/m208: a datastore plan region keeps the ready card's anatomy while its
// resource or catalog loads — title, description, one row per catalog tier in
// the ready plan list (six Postgres plans since w2/m171, nine Key Value ones
// with their connection-limit column since w6/m459), and the bordered
// Cancel/Save footer — and stays inert.
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
    expect(KEY_VALUE_PLAN_SLOTS).toBe(catalogSize("valkey"));
  });
});

describe("datastore plan preview", () => {
  // Postgres's card ends in Render's Storage row (w2/m307), its Edit inert too.
  // Render's Postgres cards show no connection limit; its Key Value cards do.
  for (const [name, Skeleton, title, buttonLabels, tiers, specs] of [
    [
      "Postgres",
      DatabaseOverviewSkeleton,
      "Compute plan",
      ["Cancel", "Save", "Edit"],
      6,
      2,
    ],
    [
      "Key Value",
      KeyValueOverviewSkeleton,
      "Compute plan",
      ["Cancel", "Save"],
      9,
      3,
    ],
  ] as const) {
    it(`${name} detail skeleton previews the ready plan card`, () => {
      const { container } = render(<Skeleton />);
      const region = planRegion(container);
      const card = region.firstElementChild as HTMLElement;
      expect(card.hasAttribute("inert")).toBe(true);
      expect(within(region).getByText(title)).toBeTruthy();
      const list = region.querySelector('[aria-hidden="true"]') as HTMLElement;
      expect(list.className).toBe(COMPUTE_PLAN_LIST_CLASS);
      const rows = Array.from(list.children) as HTMLElement[];
      expect(rows).toHaveLength(tiers);
      for (const row of rows) {
        // The ready row's box: radio, name and specs, price.
        expect(row.className).toContain(COMPUTE_PLAN_ROW_CLASS);
        expect(row.children).toHaveLength(3);
        const [, body] = Array.from(row.children);
        const [nameLine, specLine] = Array.from(body.children);
        expect(nameLine.className).toContain("h-6"); // 24px name line
        expect(specLine.className).toContain("h-5"); // 20px specs line
        // CPU and RAM; on Key Value the connection limit too.
        expect(specLine.children).toHaveLength(specs);
      }
      const buttons = within(region).getAllByRole("button", { hidden: true });
      expect(buttons.map((b) => b.textContent)).toEqual(buttonLabels);
      expect(buttons.every((b) => (b as HTMLButtonElement).disabled)).toBe(
        true,
      );
      expect(within(region).queryByRole("radio", { hidden: true })).toBeNull();
    });
  }
});
