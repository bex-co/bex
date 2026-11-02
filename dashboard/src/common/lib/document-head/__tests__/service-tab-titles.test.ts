import { describe, expect, it } from "vitest";
import "@/i18n/init";
import { Route as DeploysRoute } from "@/routes/services.$serviceId.deploys.index";
import { Route as DiskRoute } from "@/routes/services.$serviceId.disk";
import { Route as EnvRoute } from "@/routes/services.$serviceId.env";
import { Route as EventsRoute } from "@/routes/services.$serviceId.events";
import { Route as LogsRoute } from "@/routes/services.$serviceId.logs";
import { Route as MetricsRoute } from "@/routes/services.$serviceId.metrics";
import { Route as PlanRoute } from "@/routes/services.$serviceId.plan";
import { Route as ScalingRoute } from "@/routes/services.$serviceId.scaling";
import { Route as SettingsRoute } from "@/routes/services.$serviceId.settings";
import { Route as ShellRoute } from "@/routes/services.$serviceId.shell";
import { Route as StaticHeadersRoute } from "@/routes/static.$serviceId.headers";

// w4/m249: the service layout sets one title for all of its tabs; each tab now
// puts its own name first.
const tabs: Array<[string, { options: { head?: unknown } }]> = [
  ["Deploys", DeploysRoute],
  ["Disk", DiskRoute],
  ["Environment", EnvRoute],
  ["Events", EventsRoute],
  ["Logs", LogsRoute],
  ["Metrics", MetricsRoute],
  ["Plan", PlanRoute],
  ["Scaling", ScalingRoute],
  ["Settings", SettingsRoute],
  ["Shell", ShellRoute],
];

type HeadFn = (ctx: { matches: unknown[] }) => {
  meta: Array<{ title?: string }>;
};
const titleOf = (route: { options: { head?: unknown } }, loaderData: unknown) =>
  (route.options.head as HeadFn)({
    matches: [{ routeId: "/services/$serviceId", loaderData }],
  }).meta[0]?.title;

const ready = {
  state: "ready",
  resource: { name: "api", displayName: "", type: "web_service" },
};

describe("service tab titles", () => {
  it("starts each tab's title with its own name, all distinct", () => {
    const titles = tabs.map(([, route]) => titleOf(route, ready));
    tabs.forEach(([name], i) => {
      expect(titles[i]).toMatch(new RegExp(`^${name}\\b`));
      expect(titles[i]).toContain("api");
    });
    expect(new Set(titles).size).toBe(tabs.length);
  });

  it("falls back to the state title while the service is not ready", () => {
    expect(titleOf(LogsRoute, { state: "not-found" })).not.toMatch(/^Logs/);
    expect(titleOf(LogsRoute, undefined)).not.toMatch(/^Logs/);
  });

  it("covers a static site's tabs too", () => {
    const head = (StaticHeadersRoute.options.head as HeadFn)({
      matches: [{ routeId: "/static/$serviceId", loaderData: ready }],
    });
    expect(head.meta[0]?.title).toMatch(/^Headers\b/);
  });
});
