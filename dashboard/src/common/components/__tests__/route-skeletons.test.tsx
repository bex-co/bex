import { render, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it } from "vitest";
import {
  AccountSettingsPageSkeleton,
  AuthWidgetSkeleton,
  BillingPageSkeleton,
  BlueprintCreatePageSkeleton,
  DatabaseOverviewSkeleton,
  DatastoreLogsSkeleton,
  EnvGroupDetailContentSkeleton,
  EnvGroupsListPageSkeleton,
  KeyValueCreatePageSkeleton,
  KeyValueOverviewSkeleton,
  ServiceEnvironmentSkeleton,
  ServiceCreatePageSkeleton,
  ServiceLogsSkeleton,
  ServiceMetricsSkeleton,
  ServiceRouteContentSkeleton,
  ServiceSettingsSkeleton,
  StaticEdgeRulesSkeleton,
  WebhookCreatePageSkeleton,
  WebhookSettingsSkeleton,
  WorkspaceSettingsPageSkeleton,
} from "@/common/components/route-skeletons";

function regions(container: HTMLElement): string[] {
  return [...container.querySelectorAll<HTMLElement>("[data-skeleton-region]")]
    .map((element) => element.dataset.skeletonRegion)
    .filter((name): name is string => Boolean(name));
}

function renderServiceCreateSkeleton(path: string) {
  const rootRoute = createRootRoute();
  const createRouteNode = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/new",
    component: ServiceCreatePageSkeleton,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([createRouteNode]),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: {},
  });
  return render(<RouterProvider router={router} />);
}

function renderServiceRootSkeleton(base: "/services" | "/static") {
  const rootRoute = createRootRoute({ component: Outlet });
  const resourceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: base === "/services" ? "/services/$serviceId" : "/static/$serviceId",
    component: Outlet,
  });
  const indexRoute = createRoute({
    getParentRoute: () => resourceRoute,
    path: "/",
    component: () => <ServiceRouteContentSkeleton base={base} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([resourceRoute.addChildren([indexRoute])]),
    history: createMemoryHistory({ initialEntries: [`${base}/srv-1`] }),
    context: {},
  });
  return render(<RouterProvider router={router} />);
}

describe("route-shaped skeleton geometry (w5/m79)", () => {
  it.each([
    [
      <BillingPageSkeleton key="billing" />,
      "billing",
      ["page-header", "plan", "charges", "invoice-history"],
    ],
    [
      <EnvGroupsListPageSkeleton key="env-groups" />,
      "env-groups-list",
      ["page-header", "search", "env-groups-table"],
    ],
    [
      <ServiceEnvironmentSkeleton key="environment" />,
      "service-environment",
      ["page-header", "environment-editor", "environment-groups"],
    ],
    [
      <ServiceLogsSkeleton key="logs" />,
      "service-logs",
      ["log-filters", "log-panel"],
    ],
    [
      <ServiceMetricsSkeleton key="metrics" />,
      "service-metrics",
      ["metrics-filters", "application-metrics", "network-metrics"],
    ],
    [
      <EnvGroupDetailContentSkeleton key="env-group-detail" />,
      "env-group-detail",
      ["metadata", "environment-editor", "linked-services"],
    ],
    [
      <DatastoreLogsSkeleton key="datastore-logs" />,
      "datastore-logs",
      ["log-filters", "log-lines"],
    ],
    [
      <BlueprintCreatePageSkeleton key="blueprint-create" />,
      "blueprint-create",
      ["form-header", "source-picker", "settings", "preview", "actions"],
    ],
    [
      <KeyValueCreatePageSkeleton key="keyvalue-create" />,
      "keyvalue-create",
      ["plan-picker", "project-environment", "public-access", "actions"],
    ],
    [
      <WebhookCreatePageSkeleton key="webhook-create" />,
      "webhook-create",
      ["page-header", "identity", "events", "status", "actions"],
    ],
    [
      <WebhookSettingsSkeleton key="webhook-settings" />,
      "webhook-settings",
      ["settings-general", "settings-events", "danger-zone"],
    ],
    [
      <StaticEdgeRulesSkeleton key="edge-rules" />,
      "static-edge-rules",
      ["rules-editor"],
    ],
  ])("renders the %s contract", (component, shape, expectedRegions) => {
    const { container, unmount } = render(component);
    const frame = container.querySelector<HTMLElement>(
      `[data-route-skeleton="${shape}"]`,
    );
    expect(frame).not.toBeNull();
    for (const region of expectedRegions) {
      expect(regions(frame!)).toContain(region);
    }
    unmount();
  });

  it.each([
    [<DatabaseOverviewSkeleton key="database" />, 10],
    [<KeyValueOverviewSkeleton key="keyvalue" />, 6],
    [<WorkspaceSettingsPageSkeleton key="workspace" />, 3],
    [<ServiceSettingsSkeleton key="service" />, 11],
  ])(
    "keeps long-page section navigation at narrow and wide breakpoints",
    (component, count) => {
      const { container, unmount } = render(component);
      const nav = container.querySelector<HTMLElement>(
        '[data-skeleton-region="section-navigation"]',
      );

      expect(nav).not.toBeNull();
      expect(nav).toHaveClass("sticky", "lg:col-start-2");
      expect(nav).toHaveClass("min-w-0");
      expect(nav).not.toHaveClass("max-w-full");
      expect(nav).not.toHaveClass("hidden");
      expect(nav!.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(
        count,
      );
      unmount();
    },
  );

  it("matches the compact ready-state Source card instead of an editable form", () => {
    const { container } = render(<ServiceSettingsSkeleton />);
    const source = container.querySelector<HTMLElement>(
      '[data-skeleton-region="source"]',
    );
    const fields = source?.querySelector<HTMLElement>(
      '[data-skeleton-region="source-fields"]',
    );

    expect(fields).not.toBeNull();
    expect(fields).toHaveClass("grid", "sm:grid-cols-2", "text-sm");
    expect(fields?.children).toHaveLength(2);
    expect(fields?.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(4);
    expect(fields?.querySelector(".h-9")).toBeNull();
  });

  it("keeps account settings mobile navigation and desktop rail together", () => {
    const { container } = render(<AccountSettingsPageSkeleton />);
    expect(regions(container)).toEqual(
      expect.arrayContaining(["mobile-navigation", "section-navigation"]),
    );
  });

  it.each([
    [<BlueprintCreatePageSkeleton key="blueprint" />, "min-h-[902px]"],
    [<KeyValueCreatePageSkeleton key="keyvalue" />, "min-h-[1190px]"],
    [<WebhookCreatePageSkeleton key="webhook" />, "min-h-[973px]"],
  ])(
    "reserves the measured narrow-mobile create-card height",
    (component, heightClass) => {
      const { container, unmount } = render(component);
      expect(container.querySelector('[data-slot="card"]')).toHaveClass(
        heightClass,
        "sm:min-h-0",
      );
      unmount();
    },
  );

  it("uses editor-card geometry for the service environment skeleton", () => {
    const { container } = render(<ServiceEnvironmentSkeleton />);
    expect(container.querySelectorAll('[data-slot="card"]')).toHaveLength(3);
  });

  it.each([
    ["/services", "deploys-list"],
    ["/static", "service-events"],
  ] as const)(
    "selects the canonical root destination for %s",
    async (base, expectedShape) => {
      const { container } = renderServiceRootSkeleton(base);
      await waitFor(() =>
        expect(
          container.querySelector(`[data-route-skeleton="${expectedShape}"]`),
        ).not.toBeNull(),
      );
    },
  );

  it("uses a full Ory-card silhouette for in-page flow loading", () => {
    const { container } = render(<AuthWidgetSkeleton fields={3} />);
    const widget = container.querySelector(
      '[data-skeleton-region="auth-widget"]',
    );
    expect(widget).toHaveClass("rounded-xl", "border", "p-6");
    expect(
      widget!.querySelectorAll('[data-slot="skeleton"]').length,
    ).toBeGreaterThan(6);
  });

  it("keeps static metrics free of the application-metrics region", () => {
    const { container } = render(<ServiceMetricsSkeleton staticSite />);
    expect(regions(container)).toContain("network-metrics");
    expect(regions(container)).not.toContain("application-metrics");
  });

  it.each([
    ["/services/new", "plan-picker", "build-filters"],
    ["/services/new?type=static_site", "build-filters", "plan-picker"],
  ])(
    "selects the service-create geometry for %s",
    async (path, expected, absent) => {
      const { container } = renderServiceCreateSkeleton(path);

      await waitFor(() =>
        expect(
          container.querySelector('[data-route-skeleton="service-create"]'),
        ).not.toBeNull(),
      );
      expect(regions(container)).toContain(expected);
      expect(regions(container)).not.toContain(absent);
      expect(regions(container)).toEqual(
        expect.arrayContaining([
          "service-type",
          "source-picker",
          "project-environment",
          "environment-variables",
          "secret-files",
          "actions",
        ]),
      );
      const card = container.querySelector('[data-slot="card"]');
      if (path.endsWith("static_site")) {
        expect(card).not.toHaveClass("min-h-[2458px]");
      } else {
        expect(card).toHaveClass("min-h-[2458px]", "sm:min-h-0");
      }
    },
  );
});
