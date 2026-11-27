import { describe, expect, it } from "vitest";
import { serviceSubpathLanding } from "../service-subpaths";

// w3/m707: Render's service sub-paths whose name or shape is not bex's.
describe("serviceSubpathLanding", () => {
  it("opens Render's /disks on bex's Disk page", () => {
    expect(serviceSubpathLanding("srv-a", "disks")).toEqual({ page: "/disk" });
  });

  it.each([
    ["5m", "30m"],
    ["15m", "30m"],
    ["30m", "30m"],
    ["1h", "1h"],
    ["4h", "4h"],
    ["12h", "12h"],
    ["24h", "24h"],
    ["2d", "2d"],
    ["1w", "7d"],
  ])(
    "opens Render's /metrics/%s on the window its period table names (%s)",
    (period, range) => {
      expect(serviceSubpathLanding("srv-a", `metrics/${period}`)).toEqual({
        page: "/metrics",
        search: { range },
      });
    },
  );

  it("opens Metrics on its default window for a period Render's table lacks", () => {
    for (const period of ["7d", "14d", "forever", "constructor"]) {
      expect(serviceSubpathLanding("srv-a", `metrics/${period}`)).toEqual({
        page: "/metrics",
        search: undefined,
      });
    }
  });

  it("opens Shell on the instance Render names by its id's last segment", () => {
    expect(serviceSubpathLanding("srv-a", "shell/lqtnn")).toEqual({
      page: "/shell",
      search: { instance: "srv-a-lqtnn" },
    });
  });

  it("keeps a whole instance id as it is", () => {
    expect(serviceSubpathLanding("srv-a", "/shell/srv-a-lqtnn/")).toEqual({
      page: "/shell",
      search: { instance: "srv-a-lqtnn" },
    });
  });

  it("opens a deploy's page for any of its sub-paths, as Render's deploys/:deployId/* does", () => {
    expect(serviceSubpathLanding("srv-a", "deploys/dep-b/logs")).toEqual({
      page: "/deploys/$deployId",
      deployId: "dep-b",
    });
  });

  it.each([
    "",
    "jobs",
    "previews",
    "pulls",
    "sharing",
    "no-such-page",
    "disks/extra",
    "metrics/1h/extra",
    "shell/lqtnn/extra",
    "logs/extra",
  ])("sends %j to the service's root, as Render's * route does", (splat) => {
    expect(serviceSubpathLanding("srv-a", splat)).toEqual({ page: null });
  });
});
