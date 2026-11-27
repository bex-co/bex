import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { createMemoryHistory, createRouter } from "@tanstack/react-router";
import { routeTree } from "@/routeTree.gen";

// w3/m705: every URL Render's dashboard builds or routes has a row in
// dashboard-routes.md's URL table (w3/m704, checked weekly against Render's
// bundle). A row bex serves, `matched` or `partial`, must reach a route of its
// own in the real route tree, never the not-found catch-all `routes/$.tsx`.

const LEDGER = join(
  import.meta.dirname,
  "../../../../docs/render-artifacts/dashboard-routes.md",
);

interface LedgerRow {
  url: string;
  disposition: string;
}

function ledgerRows(): LedgerRow[] {
  const text = readFileSync(LEDGER, "utf8");
  const table = text.slice(text.indexOf("## Render's URL inventory"));
  const rows: LedgerRow[] = [];
  for (const line of table.split("\n")) {
    const cells = line.split(" | ");
    const url = /^\| `([^`]+)`$/.exec(cells[0] ?? "")?.[1];
    if (url && cells.length >= 4) rows.push({ url, disposition: cells[2] });
  }
  return rows;
}

/**
 * A path the pattern stands for: the hash dropped (no route sees it), a
 * trailing `*` as no sub-path, a leading `:param` as a service segment (the
 * only first segment Render's patterns leave to run time), and every other
 * `:param` as an id.
 */
function examplePath(pattern: string): string {
  const path = pattern.replace(/#.*$/, "").replace(/\/\*$/, "");
  return (
    path
      .split("/")
      .map((segment, index) =>
        segment === ":param"
          ? index === 1
            ? "web"
            : "xxx-dddddddddddddddddddd"
          : segment,
      )
      .join("/") || "/"
  );
}

describe("Render's dashboard URLs bex serves (dashboard-routes.md)", () => {
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  const served = ledgerRows().filter(({ disposition }) =>
    ["matched", "partial"].includes(disposition),
  );

  it("reads the URL table", () => {
    // The table had 385 rows when w3/m704 wrote it; a parse that finds few
    // would let this suite pass on nothing.
    expect(ledgerRows().length).toBeGreaterThan(300);
    expect(served.length).toBeGreaterThan(150);
  });

  const answers = (path: string) => {
    const [, , found] = router.getMatchedRoutes(path);
    return found !== undefined && found.id !== "/$";
  };

  it.each(served.map(({ url }) => [url, examplePath(url)]))(
    "routes %s (as %s) to a page or an alias, not the catch-all",
    (url, path) => {
      // A trailing `*` stands for the pages under a prefix (a sidebar's
      // pattern): bex answers it when it answers the prefix, or a page under
      // it the table lists.
      const under = url.endsWith("/*") ? url.slice(0, -1) : undefined;
      const listed = under
        ? served
            .filter((row) => row.url !== url && row.url.startsWith(under))
            .map((row) => examplePath(row.url))
        : [];
      expect([path, ...listed].some(answers)).toBe(true);
    },
  );

  // The routes w3/m705 added must win in the real tree, not only in a
  // hand-built router: each URL names the route that answers it.
  it.each([
    ["/env-group/evg-x", "/env-group/$"],
    ["/env-groups/new", "/env-groups_/new"],
    ["/env-groups/evg-x", "/env-groups_/$groupId"],
    ["/new", "/new/$"],
    ["/new/web", "/new/$"],
    ["/new/database", "/new/database"],
    ["/new/env-group", "/new/env-group"],
    ["/postgres/new", "/$serviceTypeSlug/new"],
    ["/web/new", "/_serviceCreate/web/new"],
    ["/blueprint/new", "/blueprint/new"],
    ["/blueprints/new", "/blueprints/new"],
    ["/services/new", "/services/new"],
    ["/keyvalue/new", "/keyvalue/new"],
    ["/settings", "/settings"],
    ["/settings/spend-limit", "/settings_/spend-limit"],
    ["/network-access", "/network-access/$"],
    ["/t/tea-x/settings", "/t/$"],
    ["/organization/org-x/billing", "/organization/$"],
    ["/sandbox-group/new", "/sandbox-group/$"],
    ["/resource/srv-dddddddddddddddddddd", "/resource/$"],
    ["/build-sources/bds-x/builds", "/build-sources_/$buildSourceId/builds/"],
  ])("answers %s with %s", (path, routeId) => {
    const [, , found] = router.getMatchedRoutes(path);
    expect(found?.id).toBe(routeId);
  });
});
