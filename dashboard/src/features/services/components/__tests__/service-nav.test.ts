import { describe, it, expect } from "vitest";
import { serviceNavGroups, serviceTabLabelKey } from "../service-nav";
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
  // replaces bex's Scaling and Plan entries for every runtime type.
  it.each(["web", "private", "worker", "cron"] as const)(
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

  it("cron: Builds and Runs replace Deploys, as on Render (w2/m360)", () => {
    const groups = serviceNavGroups("cron", "/services");
    expect(labels(groups)[0]).toEqual([
      "services.navBuilds",
      "services.navRuns",
      "services.navSettings",
    ]);
    expect(tos(groups)).toContain("/services/$serviceId/builds");
    expect(tos(groups)).toContain("/services/$serviceId/runs");
    expect(tos(groups)).not.toContain("/services/$serviceId/deploys");
    // Only a cron job has them; the type-unknown frame shows neither.
    for (const type of ["web", "private", "worker", "static", null] as const) {
      const flat = labels(serviceNavGroups(type)).flat();
      expect(flat).not.toContain("services.navBuilds");
      expect(flat).not.toContain("services.navRuns");
    }
  });

  it("cron omits Disk (no long-running instance) and keeps Compute for its plan", () => {
    const flat = labels(serviceNavGroups("cron")).flat();
    expect(flat).toContain("services.navShell");
    expect(flat).toContain("services.navCompute");
    expect(flat).not.toContain("services.navDisk");
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
});

describe("serviceTabLabelKey (the detail layout's tab heading)", () => {
  it("names the tab a path is exactly on with its sidebar label", () => {
    expect(serviceTabLabelKey("/services/srv-1/logs", "srv-1")).toBe(
      "services.navLogs",
    );
    expect(serviceTabLabelKey("/services/srv-1/env/", "srv-1")).toBe(
      "services.navEnvironment",
    );
    // A cron job's Builds and a static site's Headers are tabs too.
    expect(serviceTabLabelKey("/services/srv-1/builds", "srv-1")).toBe(
      "services.navBuilds",
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
