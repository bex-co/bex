import { describe, it, expect } from "vitest";
import { serviceNavGroups } from "../service-nav";
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
    // Every link stays within the /static tree; no Deploys/Logs/Shell/Scaling/Plan.
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
        "services.navScaling",
        "services.navPlan",
      ],
    ]);
    expect(
      tos(groups).every((to) => to.startsWith("/services/$serviceId")),
    ).toBe(true);
  });

  it("null (still loading): only the shared entries, so nothing type-specific flashes", () => {
    const groups = serviceNavGroups(null, "/services");
    const flat = labels(groups).flat();
    // Deploys and Shell/Scaling/Plan wait for the type — a static site must
    // never flash a Deploys tab (w5/m57).
    expect(groups[0].items.map((i) => i.labelKey)).toEqual([
      "services.navSettings",
    ]);
    expect(flat).not.toContain("services.navDeploys");
    expect(flat).not.toContain("services.navShell");
  });

  it("defaults the base to /services", () => {
    expect(serviceNavGroups("web")[0].items[0].to).toBe(
      "/services/$serviceId/deploys",
    );
  });
});
