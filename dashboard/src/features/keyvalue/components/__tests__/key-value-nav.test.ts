import { describe, expect, it } from "vitest";
import type { SidebarNavGroup } from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  isKeyValueNavItemActive,
  keyValueNavGroups,
  keyValuePageLabelKey,
} from "../key-value-nav";

const shape = (groups: SidebarNavGroup[]) =>
  groups.map((group) => [
    group.labelKey ?? null,
    group.items.map((item) => item.labelKey),
  ]);

// w3/m649: Render's Key Value sidebar, Info / Monitor: Logs, Metrics
// (docs/render-artifacts/key-value.md § Pages).
describe("keyValueNavGroups (w3/m649)", () => {
  it("is Render's grouping, in its order, every link under the instance", () => {
    const groups = keyValueNavGroups();
    expect(shape(groups)).toEqual([
      [null, ["keyvalue.navInfo"]],
      ["common.navMonitorGroup", ["keyvalue.navLogs", "keyvalue.navMetrics"]],
    ]);
    expect(
      groups.flatMap((group) => group.items.map((item) => item.to)),
    ).toEqual([
      "/keyvalue/$keyValueId/info",
      "/keyvalue/$keyValueId/logs",
      "/keyvalue/$keyValueId/metrics",
    ]);
  });

  it("drops Logs for a logs denial, as Render does without viewRedisLogs", () => {
    expect(shape(keyValueNavGroups({ logsDenied: true }))).toEqual([
      [null, ["keyvalue.navInfo"]],
      ["common.navMonitorGroup", ["keyvalue.navMetrics"]],
    ]);
  });

  it("drops Metrics for an activity denial, as Render does without viewRedisMetrics", () => {
    expect(shape(keyValueNavGroups({ metricsDenied: true }))).toEqual([
      [null, ["keyvalue.navInfo"]],
      ["common.navMonitorGroup", ["keyvalue.navLogs"]],
    ]);
  });

  it("drops the Monitor group once both are denied", () => {
    expect(
      shape(keyValueNavGroups({ logsDenied: true, metricsDenied: true })),
    ).toEqual([[null, ["keyvalue.navInfo"]]]);
  });
});

describe("the open Key Value page (w3/m649)", () => {
  it.each([
    ["/keyvalue/red-1", "keyvalue.navInfo"],
    ["/keyvalue/red-1/", "keyvalue.navInfo"],
    ["/keyvalue/red-1/info", "keyvalue.navInfo"],
    ["/keyvalue/red-1/logs", "keyvalue.navLogs"],
    ["/keyvalue/red-1/metrics", "keyvalue.navMetrics"],
  ])("names %s with its sidebar label", (pathname, labelKey) => {
    expect(keyValuePageLabelKey(pathname, "red-1")).toBe(labelKey);
  });

  it.each([
    "/keyvalue/red-2/logs",
    "/keyvalue/red-10/logs",
    "/keyvalue/red-1/plan",
    "/databases/red-1/logs",
  ])("names no page on %s", (pathname) => {
    expect(keyValuePageLabelKey(pathname, "red-1")).toBeNull();
  });

  it("opens exactly one entry", () => {
    const entries = keyValueNavGroups().flatMap((group) => group.items);
    const open = (pathname: string) =>
      entries
        .filter((item) => isKeyValueNavItemActive(pathname, "red-1", item.to))
        .map((item) => item.labelKey);
    expect(open("/keyvalue/red-1")).toEqual(["keyvalue.navInfo"]);
    expect(open("/keyvalue/red-1/info")).toEqual(["keyvalue.navInfo"]);
    expect(open("/keyvalue/red-1/logs")).toEqual(["keyvalue.navLogs"]);
    expect(open("/keyvalue/red-1/metrics")).toEqual(["keyvalue.navMetrics"]);
    expect(open("/keyvalue/red-1/plan")).toEqual([]);
  });
});
