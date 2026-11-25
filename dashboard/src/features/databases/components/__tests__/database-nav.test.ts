import { describe, expect, it } from "vitest";
import type { SidebarNavGroup } from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  databaseNavGroups,
  databasePageLabelKey,
  isDatabaseNavItemActive,
} from "../database-nav";

const shape = (groups: SidebarNavGroup[]) =>
  groups.map((group) => [
    group.labelKey ?? null,
    group.items.map((item) => item.labelKey),
  ]);

// w3/m641: Render's Postgres sidebar, Info / Monitor: Logs, Metrics / Manage:
// Plan, Apps, Recovery (docs/render-artifacts/postgres-dashboard.md).
describe("databaseNavGroups (w3/m641)", () => {
  it("is Render's grouping, in its order, every link under the database", () => {
    const groups = databaseNavGroups();
    expect(shape(groups)).toEqual([
      [null, ["databases.navInfo"]],
      ["common.navMonitorGroup", ["databases.navLogs", "databases.navMetrics"]],
      [
        "common.navManageGroup",
        ["databases.navPlan", "databases.navApps", "databases.navRecovery"],
      ],
    ]);
    expect(
      groups.flatMap((group) => group.items.map((item) => item.to)),
    ).toEqual([
      "/databases/$databaseId/info",
      "/databases/$databaseId/logs",
      "/databases/$databaseId/metrics",
      "/databases/$databaseId/plan",
      "/databases/$databaseId/apps",
      "/databases/$databaseId/recovery",
    ]);
  });

  it("drops Logs for a logs denial, as Render does without viewDatabaseLogs", () => {
    expect(shape(databaseNavGroups({ logsDenied: true }))[1]).toEqual([
      "common.navMonitorGroup",
      ["databases.navMetrics"],
    ]);
  });

  it("drops Metrics for an activity denial, as Render does without viewDatabaseMetrics", () => {
    expect(shape(databaseNavGroups({ metricsDenied: true }))[1]).toEqual([
      "common.navMonitorGroup",
      ["databases.navLogs"],
    ]);
  });

  it("drops a group left empty", () => {
    expect(
      shape(databaseNavGroups({ logsDenied: true, metricsDenied: true })),
    ).toEqual([
      [null, ["databases.navInfo"]],
      [
        "common.navManageGroup",
        ["databases.navPlan", "databases.navApps", "databases.navRecovery"],
      ],
    ]);
  });
});

describe("the open database page (w3/m641)", () => {
  it.each([
    ["/databases/dpg-1", "databases.navInfo"],
    ["/databases/dpg-1/", "databases.navInfo"],
    ["/databases/dpg-1/info", "databases.navInfo"],
    ["/databases/dpg-1/logs", "databases.navLogs"],
    ["/databases/dpg-1/metrics", "databases.navMetrics"],
    ["/databases/dpg-1/plan", "databases.navPlan"],
    ["/databases/dpg-1/high-availability-upgrade", "databases.navPlan"],
    ["/databases/dpg-1/apps", "databases.navApps"],
    ["/databases/dpg-1/recovery", "databases.navRecovery"],
  ])("names %s with its sidebar label", (pathname, labelKey) => {
    expect(databasePageLabelKey(pathname, "dpg-1")).toBe(labelKey);
  });

  it.each([
    "/databases/dpg-2/plan",
    "/databases/dpg-10/plan",
    "/databases/dpg-1/sharing",
    "/services/dpg-1/plan",
  ])("names no page on %s", (pathname) => {
    expect(databasePageLabelKey(pathname, "dpg-1")).toBeNull();
  });

  it("opens exactly one entry", () => {
    const entries = databaseNavGroups().flatMap((group) => group.items);
    const open = (pathname: string) =>
      entries
        .filter((item) => isDatabaseNavItemActive(pathname, "dpg-1", item.to))
        .map((item) => item.labelKey);
    expect(open("/databases/dpg-1")).toEqual(["databases.navInfo"]);
    expect(open("/databases/dpg-1/upgrade")).toEqual(["databases.navPlan"]);
    expect(open("/databases/dpg-1/recovery")).toEqual([
      "databases.navRecovery",
    ]);
    expect(open("/databases/dpg-1/sharing")).toEqual([]);
  });
});
