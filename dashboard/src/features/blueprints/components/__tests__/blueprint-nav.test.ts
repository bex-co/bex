import { describe, expect, it } from "vitest";
import {
  BLUEPRINT_NAV_GROUPS,
  blueprintPageLabelKey,
  isBlueprintNavItemActive,
} from "../blueprint-nav";

// w3/m677: Render's Blueprint sidebar, Resources / Syncs / Settings in one
// group with no label (docs/render-artifacts/blueprint-pages.md).
describe("BLUEPRINT_NAV_GROUPS (w3/m677)", () => {
  it("is Render's three entries, in its order, every link under the Blueprint", () => {
    expect(BLUEPRINT_NAV_GROUPS).toHaveLength(1);
    const [group] = BLUEPRINT_NAV_GROUPS;
    expect(group.labelKey).toBeUndefined();
    expect(group.items.map((item) => [item.labelKey, item.to])).toEqual([
      ["blueprints.navResources", "/blueprints/$blueprintId/resources"],
      ["blueprints.navSyncs", "/blueprints/$blueprintId/syncs"],
      ["blueprints.navSettings", "/blueprints/$blueprintId/settings"],
    ]);
  });
});

describe("the open Blueprint page (w3/m677)", () => {
  it.each([
    ["/blueprints/exs-1", "blueprints.navResources"],
    ["/blueprints/exs-1/", "blueprints.navResources"],
    ["/blueprints/exs-1/resources", "blueprints.navResources"],
    ["/blueprints/exs-1/syncs", "blueprints.navSyncs"],
    // A sync's own page (w3/m678) keeps Syncs open, as Render's hrefPatterns do.
    ["/blueprints/exs-1/sync", "blueprints.navSyncs"],
    ["/blueprints/exs-1/sync/exe-9", "blueprints.navSyncs"],
    ["/blueprints/exs-1/settings", "blueprints.navSettings"],
  ])("%s is %s", (pathname, labelKey) => {
    expect(blueprintPageLabelKey(pathname, "exs-1")).toBe(labelKey);
  });

  it.each([
    ["/blueprints/exs-1/syncsx"],
    ["/blueprints/exs-1/settings/extra"],
    ["/blueprints/exs-2/syncs"],
    ["/blueprints"],
    ["/keyvalue/exs-1/syncs"],
  ])("names no page off the Blueprint's pages: %s", (pathname) => {
    expect(blueprintPageLabelKey(pathname, "exs-1")).toBeNull();
  });

  it("highlights exactly one entry, with the id as the URL spells it", () => {
    const active = (pathname: string, id: string) =>
      BLUEPRINT_NAV_GROUPS[0].items
        .filter((item) => isBlueprintNavItemActive(pathname, id, item.to))
        .map((item) => item.labelKey);
    expect(active("/blueprints/blp-1", "blp-1")).toEqual([
      "blueprints.navResources",
    ]);
    expect(active("/blueprints/blp-1/sync/exe-9", "blp-1")).toEqual([
      "blueprints.navSyncs",
    ]);
    expect(active("/blueprints/blp-1/settings", "exs-1")).toEqual([]);
  });
});
