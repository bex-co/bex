import { describe, expect, it } from "vitest";
import { Route as IndexRoute } from "@/routes/databases.$databaseId.index";
import { Route as InfoRoute } from "@/routes/databases.$databaseId.info";
import { Route as LogsRoute } from "@/routes/databases.$databaseId.logs";
import { Route as MetricsRoute } from "@/routes/databases.$databaseId.metrics";
import { Route as PlanRoute } from "@/routes/databases.$databaseId.plan";
import { Route as AppsRoute } from "@/routes/databases.$databaseId.apps";
import { Route as RecoveryRoute } from "@/routes/databases.$databaseId.recovery";

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
      { routeId: "/databases/$databaseId", loaderData, status: "success" },
    ],
  });
  return head.meta.find((entry) => entry.title)?.title;
}

const ORDERS = { state: "ready", resource: { id: "dpg-1", name: "orders" } };

// w3/m641: each of a Postgres database's pages has its own document title,
// its name leading Render's "orders ・ Database", as a service tab's does.
describe("a Postgres database page's document title (w3/m641)", () => {
  it.each([
    [IndexRoute, "Info"],
    [InfoRoute, "Info"],
    [LogsRoute, "Logs"],
    [MetricsRoute, "Metrics"],
    [PlanRoute, "Plan"],
    [AppsRoute, "Apps"],
    [RecoveryRoute, "Recovery"],
  ])("names the page first", (route, page) => {
    expect(title(route, ORDERS)).toBe(
      `${page} ・ orders ・ Database ・ bex Dashboard`,
    );
  });

  it("says loading, never a stale name, while the layout's read is in flight", () => {
    expect(title(RecoveryRoute, undefined)).toBe("Loading… ・ bex Dashboard");
  });

  it("never names a database the read did not find", () => {
    expect(title(RecoveryRoute, { state: "not-found" })).toBe(
      "Page not found ・ bex Dashboard",
    );
  });
});
