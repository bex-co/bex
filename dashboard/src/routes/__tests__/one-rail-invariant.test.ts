import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * The one-rail invariant (w5/m64).
 *
 * The dashboard has exactly ONE left rail: `DashboardSidebar`, which swaps its
 * own contents by route (`ProjectSidebar`/`ServiceSidebar` replace the nav;
 * the agent-sessions section augments it). A page must never render a rail of
 * its own inside `DashboardLayout` — that produces two side-by-side sidebars.
 *
 * `/agents` shipped exactly that bug: `agents.tsx` and
 * `agents_.$agentSessionId.tsx` each mounted a bespoke `<aside>` on top of the
 * layout's rail. Nothing asserted how many rails a page rendered, so it reached
 * production. This guard is that missing assertion — structural on purpose, so
 * it catches the mistake at its source rather than after a screenshot.
 *
 * If a page genuinely needs a second panel, put it on the RIGHT (Devin's own
 * answer — its session view pairs the chat column with a right-hand workspace
 * panel) and give it a non-`aside` container.
 */
const ROUTES_DIR = join(import.meta.dirname, "..");

function routeFiles(): string[] {
  return readdirSync(ROUTES_DIR)
    .filter((f) => f.endsWith(".tsx") && !f.startsWith("api."))
    .sort();
}

describe("one-rail invariant", () => {
  it("no route module renders its own <aside> rail", () => {
    const offenders = routeFiles().filter((file) =>
      /<aside[\s>]/.test(readFileSync(join(ROUTES_DIR, file), "utf8")),
    );
    expect(offenders).toEqual([]);
  });

  it("no route module imports a *-sidebar component", () => {
    const offenders = routeFiles().filter((file) => {
      const src = readFileSync(join(ROUTES_DIR, file), "utf8");
      return /^import\s[^;]*\bfrom\s+["'][^"']*-sidebar["']/m.test(src);
    });
    expect(offenders).toEqual([]);
  });

  it("guards a non-empty set of route files", () => {
    // Keeps the two assertions above from passing vacuously if the glob breaks.
    expect(routeFiles().length).toBeGreaterThan(20);
  });
});
