import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * Top-level list/create routes must show a content-shaped skeleton while
 * pending, not the bare `RoutePending` spinner (w9/m69). This guards that each
 * target route wires a `pendingComponent` to one of the shared content
 * skeletons — a regression to the spinner (dropping the prop) fails here.
 */
const ROUTES_DIR = join(import.meta.dirname, "..");

const LIST_ROUTES = [
  "blueprints.tsx",
  "env-groups.tsx",
  "webhooks.tsx",
  "notifications.tsx",
  "billing.tsx",
];
const CREATE_ROUTES = [
  "blueprints.new.tsx",
  "webhooks_.new.tsx",
  "new.workspace.tsx",
];

function src(file: string): string {
  return readFileSync(join(ROUTES_DIR, file), "utf8");
}

describe("list/create route pending skeletons (w9/m69)", () => {
  it("every target list route uses ListPageSkeleton as its pendingComponent", () => {
    const offenders = LIST_ROUTES.filter(
      (f) => !/pendingComponent:\s*ListPageSkeleton\b/.test(src(f)),
    );
    expect(offenders).toEqual([]);
  });

  it("agents uses a composer+recents pending skeleton, not the card grid", () => {
    const agents = src("agents.tsx");
    expect(agents).toMatch(/pendingComponent:\s*AgentsPageSkeleton\b/);
    expect(agents).not.toMatch(/pendingComponent:\s*ListPageSkeleton\b/);
  });

  it("every target create route uses FormPageSkeleton as its pendingComponent", () => {
    const offenders = CREATE_ROUTES.filter(
      (f) => !/pendingComponent:\s*FormPageSkeleton\b/.test(src(f)),
    );
    expect(offenders).toEqual([]);
  });
});
