import { describe, expect, it } from "vitest";
import { Route as IndexRoute } from "@/routes/keyvalue.$keyValueId.index";
import { Route as InfoRoute } from "@/routes/keyvalue.$keyValueId.info";
import { Route as LogsRoute } from "@/routes/keyvalue.$keyValueId.logs";
import { Route as MetricsRoute } from "@/routes/keyvalue.$keyValueId.metrics";

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
      { routeId: "/keyvalue/$keyValueId", loaderData, status: "success" },
    ],
  });
  return head.meta.find((entry) => entry.title)?.title;
}

const CACHE = { state: "ready", resource: { id: "red-1", name: "cache" } };

// w3/m649: each of a Key Value instance's pages has its own document title,
// its name leading Render's "cache ・ Key Value", as a Postgres database
// page's does (w3/m641).
describe("a Key Value page's document title (w3/m649)", () => {
  it.each([
    [IndexRoute, "Info"],
    [InfoRoute, "Info"],
    [LogsRoute, "Logs"],
    [MetricsRoute, "Metrics"],
  ])("names the page first", (route, page) => {
    expect(title(route, CACHE)).toBe(
      `${page} ・ cache ・ Key Value ・ bex Dashboard`,
    );
  });

  it("says loading, never a stale name, while the layout's read is in flight", () => {
    expect(title(LogsRoute, undefined)).toBe("Loading… ・ bex Dashboard");
  });

  it("never names an instance the read did not find", () => {
    expect(title(MetricsRoute, { state: "not-found" })).toBe(
      "Page not found ・ bex Dashboard",
    );
  });
});
