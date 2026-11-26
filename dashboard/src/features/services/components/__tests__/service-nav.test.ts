import { describe, it, expect } from "vitest";
import {
  serviceHeaderHidden,
  serviceNavGroups,
  serviceTabLabelKey,
} from "../service-nav";
import type { SidebarNavGroup } from "@/common/components/dashboard-layout/sidebar-nav-groups";

const labels = (groups: SidebarNavGroup[]) =>
  groups.map((g) => g.items.map((i) => i.labelKey));
const tos = (groups: SidebarNavGroup[]) =>
  groups.flatMap((g) => g.items.map((i) => i.to));

describe("serviceNavGroups (w5/m48, w5/m57)", () => {
  it("static_site: Events-first, no Deploys tab, edge-rule pages, all under /static", () => {
    const groups = serviceNavGroups("static", "/static");
    expect(labels(groups)).toEqual([
      ["services.navEvents", "services.navSettings"],
      ["services.navMetrics"],
      [
        "services.navEnvironment",
        "services.navRedirects",
        "services.navHeaders",
      ],
    ]);
    // Every link stays within the /static tree; no Deploys/Logs/Shell/Compute.
    expect(tos(groups).every((to) => to.startsWith("/static/$serviceId"))).toBe(
      true,
    );
    expect(tos(groups)).not.toContain("/static/$serviceId/deploys");
    expect(tos(groups)).not.toContain("/static/$serviceId/logs");
  });

  it("compute type: Deploys-first under /services with the runtime tabs", () => {
    const groups = serviceNavGroups("web", "/services");
    expect(labels(groups)).toEqual([
      ["services.navDeploys", "services.navSettings"],
      ["services.navEvents", "services.navLogs", "services.navMetrics"],
      [
        "services.navEnvironment",
        "services.navDisk",
        "services.navShell",
        "services.navCompute",
      ],
    ]);
    expect(
      tos(groups).every((to) => to.startsWith("/services/$serviceId")),
    ).toBe(true);
  });

  // w6/m483: one Compute entry, where Render's Manage group had Scaling,
  // replaces bex's Scaling and Plan entries for every type that runs
  // instances.
  it.each(["web", "private", "worker"] as const)(
    "%s has one Compute entry and neither Scaling nor Plan",
    (type) => {
      const groups = serviceNavGroups(type);
      const manage = groups[2].items;
      expect(manage.at(-1)).toMatchObject({
        labelKey: "services.navCompute",
        to: "/services/$serviceId/compute",
      });
      expect(tos(groups)).not.toContain("/services/$serviceId/plan");
      expect(tos(groups)).not.toContain("/services/$serviceId/scaling");
    },
  );

  it("worker keeps Disk (same eligible set as the API)", () => {
    const flat = labels(serviceNavGroups("worker")).flat();
    expect(flat).toContain("services.navDisk");
  });

  // Render's signed-in cron sidebar (w3/m554,
  // docs/render-artifacts/cron-dashboard.md): Runs, Deploys, Settings; Events,
  // Logs, Metrics; Environment, Shell (One-Off Jobs is a DO_NOT_DO non-goal).
  it("cron: Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell", () => {
    const groups = serviceNavGroups("cron", "/services");
    expect(labels(groups)).toEqual([
      ["services.navRuns", "services.navDeploys", "services.navSettings"],
      ["services.navEvents", "services.navLogs", "services.navMetrics"],
      ["services.navEnvironment", "services.navShell"],
    ]);
    expect(groups.map((group) => group.labelKey)).toEqual([
      undefined,
      "common.navMonitorGroup",
      "common.navManageGroup",
    ]);
    expect(tos(groups).slice(0, 3)).toEqual([
      "/services/$serviceId/runs",
      "/services/$serviceId/deploys",
      "/services/$serviceId/settings",
    ]);
    // No Builds page, no Disk (no long-running instance to mount one on) and
    // no Compute page: a cron job's plan is its Settings' Compute section.
    expect(tos(groups)).not.toContain("/services/$serviceId/builds");
    expect(tos(groups)).not.toContain("/services/$serviceId/disk");
    expect(tos(groups)).not.toContain("/services/$serviceId/compute");
  });

  it("gives only a cron job a Runs entry", () => {
    for (const type of ["web", "private", "worker", "static", null] as const) {
      expect(labels(serviceNavGroups(type)).flat()).not.toContain(
        "services.navRuns",
      );
    }
  });

  it("null (still loading): only the shared entries, so nothing type-specific flashes", () => {
    const groups = serviceNavGroups(null, "/services");
    const flat = labels(groups).flat();
    // Deploys and Shell/Compute wait for the type — a static site must
    // never flash a Deploys tab (w5/m57).
    expect(groups[0].items.map((i) => i.labelKey)).toEqual([
      "services.navSettings",
    ]);
    expect(flat).not.toContain("services.navDeploys");
    expect(flat).not.toContain("services.navShell");
    expect(flat).not.toContain("services.navCompute");
  });

  it("defaults the base to /services", () => {
    expect(serviceNavGroups("web")[0].items[0].to).toBe(
      "/services/$serviceId/deploys",
    );
  });

  // w9/m621: a Billing member views no service events (a cron job's runs
  // among them), deploys or metrics, as on Render.
  describe("for a role confirmed unable to view service activity", () => {
    const denied = { activityDenied: true };

    it("compute type: drops Deploys, Events and Metrics, keeps the rest", () => {
      expect(labels(serviceNavGroups("web", "/services", denied))).toEqual([
        ["services.navSettings"],
        ["services.navLogs"],
        [
          "services.navEnvironment",
          "services.navDisk",
          "services.navShell",
          "services.navCompute",
        ],
      ]);
    });

    it("cron: drops Runs too", () => {
      expect(labels(serviceNavGroups("cron", "/services", denied))).toEqual([
        ["services.navSettings"],
        ["services.navLogs"],
        ["services.navEnvironment", "services.navShell"],
      ]);
    });

    it("static_site: drops Events and the Monitor group Metrics leaves empty", () => {
      const groups = serviceNavGroups("static", "/static", denied);
      expect(labels(groups)).toEqual([
        ["services.navSettings"],
        [
          "services.navEnvironment",
          "services.navRedirects",
          "services.navHeaders",
        ],
      ]);
      expect(groups.map((group) => group.labelKey)).toEqual([
        undefined,
        "common.navManageGroup",
      ]);
    });

    it("leaves the tab headings alone: a denied page still names itself", () => {
      expect(serviceTabLabelKey("/services/srv-1/deploys", "srv-1")).toBe(
        "services.navDeploys",
      );
    });
  });

  // w9/145: neither a Viewer nor a Billing member views service logs, as on
  // Render ("View service logs" ❌ for both).
  describe("for a role confirmed unable to view logs", () => {
    it("a Viewer's compute type: drops Logs alone", () => {
      expect(
        labels(serviceNavGroups("web", "/services", { logsDenied: true })),
      ).toEqual([
        ["services.navDeploys", "services.navSettings"],
        ["services.navEvents", "services.navMetrics"],
        [
          "services.navEnvironment",
          "services.navDisk",
          "services.navShell",
          "services.navCompute",
        ],
      ]);
    });

    it("a Billing member's: drops the Monitor group it leaves empty", () => {
      const groups = serviceNavGroups("cron", "/services", {
        activityDenied: true,
        logsDenied: true,
      });
      expect(labels(groups)).toEqual([
        ["services.navSettings"],
        ["services.navEnvironment", "services.navShell"],
      ]);
      expect(groups.map((group) => group.labelKey)).toEqual([
        undefined,
        "common.navManageGroup",
      ]);
    });

    it("static_site: unchanged, it has no Logs to drop", () => {
      expect(
        serviceNavGroups("static", "/static", { logsDenied: true }),
      ).toEqual(serviceNavGroups("static", "/static"));
    });

    it("leaves the Logs heading alone: a deep link still names its tab", () => {
      expect(serviceTabLabelKey("/services/srv-1/logs", "srv-1")).toBe(
        "services.navLogs",
      );
    });
  });
});

describe("serviceTabLabelKey (the detail layout's tab heading)", () => {
  it("names the tab a path is exactly on with its sidebar label", () => {
    expect(serviceTabLabelKey("/services/srv-1/logs", "srv-1")).toBe(
      "services.navLogs",
    );
    expect(serviceTabLabelKey("/services/srv-1/env/", "srv-1")).toBe(
      "services.navEnvironment",
    );
    // A cron job's Runs and a static site's Headers are tabs too.
    expect(serviceTabLabelKey("/services/srv-1/runs", "srv-1")).toBe(
      "services.navRuns",
    );
    expect(serviceTabLabelKey("/services/srv-1/compute", "srv-1")).toBe(
      "services.navCompute",
    );
    expect(
      serviceTabLabelKey("/static/site-1/headers", "site-1", "/static"),
    ).toBe("services.navHeaders");
  });

  it("names nothing below a tab, off the tree, or for another service", () => {
    expect(serviceTabLabelKey("/services/srv-1/deploys/dep-1", "srv-1")).toBe(
      null,
    );
    expect(serviceTabLabelKey("/services/srv-1", "srv-1")).toBe(null);
    expect(serviceTabLabelKey("/services/srv-2/logs", "srv-1")).toBe(null);
  });
});

// w3/106: Render's service and cron layouts pass `hideResourceHeader` on
// Logs, Metrics and Shell and on a deploy's (or a build's) detail.
describe("serviceHeaderHidden (Render's hideResourceHeader)", () => {
  it.each([
    "/services/srv-1/logs",
    "/services/srv-1/logs/",
    "/services/srv-1/metrics",
    "/services/srv-1/shell",
    "/services/srv-1/deploys/dep-1",
  ])("hides the header on %s", (path) => {
    expect(serviceHeaderHidden(path, "srv-1")).toBe(true);
  });

  it("hides it on a static site's Metrics and deploy, under /static", () => {
    expect(
      serviceHeaderHidden("/static/site-1/metrics", "site-1", "/static"),
    ).toBe(true);
    expect(
      serviceHeaderHidden("/static/site-1/deploys/dep-1", "site-1", "/static"),
    ).toBe(true);
  });

  it.each([
    "/services/srv-1",
    "/services/srv-1/",
    "/services/srv-1/deploys",
    "/services/srv-1/events",
    "/services/srv-1/runs",
    "/services/srv-1/env",
    "/services/srv-1/settings",
    "/services/srv-1/compute",
    "/services/srv-1/disk",
    "/services/srv-1/logsx",
    "/services/srv-1/deploys/dep-1/extra",
    "/services/srv-2/logs",
    "/static/srv-1/logs",
  ])("keeps the header on %s", (path) => {
    expect(serviceHeaderHidden(path, "srv-1")).toBe(false);
  });
});
