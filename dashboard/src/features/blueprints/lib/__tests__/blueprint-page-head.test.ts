import { describe, expect, it } from "vitest";
import { Route as IndexRoute } from "@/routes/blueprints.$blueprintId.index";
import { Route as ResourcesRoute } from "@/routes/blueprints.$blueprintId.resources";
import { Route as SyncsRoute } from "@/routes/blueprints.$blueprintId.syncs";
import { Route as SettingsRoute } from "@/routes/blueprints.$blueprintId.settings";

type HeadContext = {
  matches: { routeId: string; loaderData?: unknown; status?: string }[];
};

function title(route: { options: { head?: unknown } }, loaderData: unknown) {
  const head = (
    route.options.head as (context: HeadContext) => {
      meta: { title?: string }[];
    }
  )({
    matches: [
      { routeId: "__root__" },
      { routeId: "/blueprints/$blueprintId", loaderData, status: "success" },
    ],
  });
  return head.meta.find((entry) => entry.title)?.title;
}

const PLATFORM = {
  state: "ready",
  resource: { id: "exs-1", name: "platform" },
};

// w3/m677: each of a Blueprint's pages has its own document title, its name
// leading Render's "platform ・ Blueprint", as bex's datastore pages do.
describe("a Blueprint page's document title (w3/m677)", () => {
  it.each([
    [IndexRoute, "Resources"],
    [ResourcesRoute, "Resources"],
    [SyncsRoute, "Syncs"],
    [SettingsRoute, "Settings"],
  ])("names the page first", (route, page) => {
    expect(title(route, PLATFORM)).toBe(
      `${page} ・ platform ・ Blueprint ・ bex Dashboard`,
    );
  });

  it("says loading, never a stale name, while the layout's read is in flight", () => {
    expect(title(SyncsRoute, undefined)).toBe("Loading… ・ bex Dashboard");
  });

  it("never names a Blueprint the read did not find", () => {
    expect(title(SettingsRoute, { state: "not-found" })).toBe(
      "Page not found ・ bex Dashboard",
    );
  });
});
