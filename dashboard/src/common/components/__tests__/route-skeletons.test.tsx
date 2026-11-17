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
  NotificationsPageSkeleton,
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

function renderServicePlanParentSkeleton(serviceType: string | null) {
  const rootRoute = createRootRoute({ component: Outlet });
  const resourceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => (
      <ServiceRouteContentSkeleton base="/services" serviceType={serviceType} />
    ),
  });
  const planRoute = createRoute({
    getParentRoute: () => resourceRoute,
    path: "/plan",
    component: () => null,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([resourceRoute.addChildren([planRoute])]),
    history: createMemoryHistory({ initialEntries: ["/services/srv-1/plan"] }),
    context: {},
  });
  return render(<RouterProvider router={router} />);
}

describe("route-shaped skeleton geometry (w5/m79)", () => {
  it.each([
    [
      <BillingPageSkeleton key="billing" />,
      "billing",
      ["page-header", "plan", "charges", "agent-model", "invoice-history"],
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
      ["page-header", "source-picker", "settings", "preview", "actions"],
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

  it("previews the create source picker's search, connection control, and repository list", async () => {
    const { container } = renderServiceCreateSkeleton(
      "/services/new?type=web_service",
    );
    await waitFor(() =>
      expect(
        container.querySelector('[data-skeleton-region="source-toolbar"]'),
      ).not.toBeNull(),
    );
    const toolbar = container.querySelector<HTMLElement>(
      '[data-skeleton-region="source-toolbar"]',
    );
    const repositories = container.querySelector<HTMLElement>(
      '[data-skeleton-region="source-repositories"]',
    );

    expect(toolbar).not.toBeNull();
    expect(toolbar).toHaveClass("flex-col", "sm:flex-row");
    expect(toolbar?.children).toHaveLength(2);
    expect(repositories?.children).toHaveLength(3);
  });

  // w9/082: the Team region is TeamPanel's own structure (static header,
  // Invite action, search, table header, one member-shaped row) and the danger
  // zone holds the Leave workspace card, so neither grows nor pops in on load.
  it("draws the Settings Team card and Leave workspace card in their loaded shapes", () => {
    const { container, unmount } = render(<WorkspaceSettingsPageSkeleton />);
    const team = container.querySelector('[data-skeleton-region="team"]')!;
    const card = team.querySelector('[data-testid="team-panel-skeleton"]')!;
    expect(card).not.toBeNull();
    expect(card.querySelector("[data-slot=card-title]")).toHaveTextContent(
      "Team",
    );
    expect(card.querySelector("[data-slot=card-action]")).toHaveTextContent(
      "Invite",
    );
    expect(
      card.querySelectorAll("[data-skeleton-region=member-row]"),
    ).toHaveLength(1);
    const danger = container.querySelector(
      '[data-skeleton-region="danger-zone"]',
    )!;
    expect(
      danger.querySelector('[data-testid="leave-workspace-skeleton"]'),
    ).toHaveTextContent("Leave workspace");
    unmount();
  });

  // w9/m388: the Inbound IP Rules region sits where the ready section does
  // (after Build Pipeline, before Team) and holds the card's own skeleton: its
  // static header, the seeded allow-all pair's two rule rows and the add row.
  it("draws the Settings Inbound IP Rules card in its loaded place and shape", () => {
    const { container, unmount } = render(<WorkspaceSettingsPageSkeleton />);
    const order = regions(container).filter((name) =>
      ["build-pipeline", "inbound-ip-rules", "team"].includes(name),
    );
    expect(order).toEqual(["build-pipeline", "inbound-ip-rules", "team"]);
    const region = container.querySelector(
      '[data-skeleton-region="inbound-ip-rules"]',
    )!;
    const card = region.querySelector(
      '[data-testid="workspace-ip-rules-skeleton"]',
    )!;
    expect(card.querySelector("[data-slot=card-title]")).toHaveTextContent(
      "Inbound IP Rules",
    );
    expect(
      card.querySelector('[data-skeleton-region="ip-rule-rows"]')?.children,
    ).toHaveLength(2);
    expect(
      card.querySelector('[data-skeleton-region="ip-rule-actions"]')?.children,
    ).toHaveLength(5);
    unmount();
  });

  it.each([
    [<DatabaseOverviewSkeleton key="database" />, 10],
    [<KeyValueOverviewSkeleton key="keyvalue" />, 7],
    [<ServiceSettingsSkeleton key="service" />, 13],
  ])(
    "keeps long-page section navigation at narrow and wide breakpoints",
    (component, count) => {
      const { container, unmount } = render(component);
      const nav = container.querySelector<HTMLElement>(
        '[data-skeleton-region="section-navigation"]',
      );

      expect(nav).not.toBeNull();
      expect(nav).toHaveClass("sticky", "@4xl/page:col-start-2");
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
    for (const field of fields?.children ?? []) {
      expect(field.firstElementChild).toHaveClass("h-5");
    }
    const heading = source?.querySelector('[data-slot="card-header"]');
    expect(heading?.lastElementChild).toHaveClass("h-8", "w-full");
    expect(heading?.querySelector(".space-y-1\\.5")?.children[0]).toHaveClass(
      "h-4",
    );
    const description = heading?.querySelector(
      '[data-slot="card-description"]',
    );
    expect(description).toHaveClass("relative");
    expect(description?.querySelector("span")).toHaveClass("invisible");
    expect(description?.querySelector("span")).toHaveTextContent(
      "The Git repository and branch this service builds from.",
    );
    expect(description?.querySelector('[data-slot="skeleton"]')).toHaveClass(
      "absolute",
      "inset-0",
    );
  });

  it("omits the branch region for a known image source", () => {
    const { container } = render(
      <ServiceSettingsSkeleton service={{ type: "web_service", repo: null }} />,
    );
    const fields = container.querySelector(
      '[data-skeleton-region="source-fields"]',
    );
    const description = container.querySelector(
      '[data-skeleton-region="source"] [data-slot="card-description"] span',
    );
    expect(description).toHaveClass("invisible");
    expect(description).toHaveTextContent(
      "The prebuilt container image this service deploys.",
    );
    expect(fields?.children).toHaveLength(1);
    expect(fields?.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(2);
  });

  it("previews an image service's Deploy card, not Build & Deploy", () => {
    const { container } = render(
      <ServiceSettingsSkeleton service={{ type: "web_service", repo: null }} />,
    );
    expect(
      container.querySelector('[data-skeleton-region="build"]'),
    ).toBeNull();
    expect(
      container.querySelector('[data-skeleton-region="deploy"]'),
    ).not.toBeNull();

    const { container: repo } = render(
      <ServiceSettingsSkeleton
        service={{ type: "web_service", repo: "https://github.com/acme/app" }}
      />,
    );
    expect(repo.querySelector('[data-skeleton-region="build"]')).not.toBeNull();
    expect(repo.querySelector('[data-skeleton-region="deploy"]')).toBeNull();
  });

  it("reserves the Docker Build Context Directory row in a Dockerfile build's Build card (w8/m128)", () => {
    const buildCard = (runtime: string) => {
      const { container } = render(
        <ServiceSettingsSkeleton
          service={{
            type: "web_service",
            repo: "https://github.com/acme/app",
            runtime,
            builder: runtime === "docker" ? "dockerfile" : "native",
          }}
        />,
      );
      const card = container.querySelector(
        '[data-skeleton-region="build"] > [data-slot="card"]',
      );
      return {
        className: card?.className ?? "",
        rows:
          card?.querySelector('[data-slot="card-content"]')?.children.length ??
          0,
      };
    };
    const docker = buildCard("docker");
    expect(docker.className).toContain("h-[930px]");
    expect(docker.className).toContain("sm:h-[778px]");
    expect(docker.rows).toBe(8);
    const native = buildCard("node");
    expect(native.className).toContain("h-[816px]");
    expect(native.rows).toBe(7);
  });

  // w3/067: each Account Settings section is its real heading over the frames
  // its cards draw while they load, not generic cards that grow on arrival.
  it("draws every Account Settings section from its cards' own loading frames", () => {
    const { container, unmount } = render(<AccountSettingsPageSkeleton />);
    const section = (name: string) =>
      container.querySelector<HTMLElement>(`[data-skeleton-region="${name}"]`)!;
    const frames = (name: string) =>
      [
        ...section(name).querySelectorAll<HTMLElement>(
          ":scope > [data-testid]",
        ),
      ].map((element) => element.dataset.testid);

    expect(
      ["account", "integrations", "access", "security", "danger-zone"].map(
        (name) => section(name).querySelector("h2")?.textContent,
      ),
    ).toEqual([
      "Account",
      "Integrations",
      "Access credentials",
      "Security",
      "Danger zone",
    ]);
    // The Kratos flow's six cards, in the order Ory draws them, each with Ory's
    // own title reserved.
    expect(frames("account")).toEqual(["account-settings-flow-skeleton"]);
    const cards = [
      ...section("account").querySelectorAll<HTMLElement>("[data-ory-group]"),
    ];
    expect(cards.map((card) => card.dataset.oryGroup)).toEqual([
      "profile",
      "password",
      "oidc",
      "lookup_secret",
      "webauthn",
      "totp",
    ]);
    expect(cards[0]).toHaveTextContent("Profile Settings");
    expect(cards[5]).toHaveTextContent("Authenticator App");
    expect(frames("integrations")).toEqual(["connect-github-card-skeleton"]);
    expect(
      section("integrations").querySelector(
        '[data-testid="git-connections-skeleton"]',
      ),
    ).not.toBeNull();
    expect(frames("access")).toEqual([
      "api-keys-panel-skeleton",
      "ssh-keys-panel-skeleton",
    ]);
    expect(frames("security")).toEqual([
      "connected-agents-panel-skeleton",
      "active-sessions-panel-skeleton",
    ]);
    expect(
      section("security").querySelectorAll('[data-testid="panel-table-frame"]'),
    ).toHaveLength(2);
    expect(
      section("security").querySelector(
        '[data-testid="current-session-placeholder"]',
      ),
    ).not.toBeNull();
    expect(frames("danger-zone")).toEqual(["account-deletion-skeleton"]);
    unmount();
  });

  it.each([
    // The test setup grants every capability, so the pending rail adds Build
    // Pipeline (billing managers', w6/m309) and Audit Logs (reserved until the
    // role loads) to the every-member sections (w3/m305), Google login
    // (w7/m465) among them.
    [
      <WorkspaceSettingsPageSkeleton key="workspace" />,
      "workspace-settings",
      12,
    ],
    [<AccountSettingsPageSkeleton key="account" />, "account-settings", 5],
    [<BillingPageSkeleton key="billing" />, "billing", 7],
    [<NotificationsPageSkeleton key="notifications" />, "notifications", 3],
  ])(
    "renders %s pending through the settings page frame, with its real rail",
    (component, shape, links) => {
      const { container, unmount } = render(component);
      const frame = container.querySelector<HTMLElement>(
        `[data-route-skeleton="${shape}"]`,
      );
      expect(frame).toHaveAttribute("data-page-scroll");
      // The ready page's own heading and navigation, not bars standing in.
      expect(
        frame!.querySelector('[data-skeleton-region="page-header"] h1'),
      ).not.toBeNull();
      const rail = frame!.querySelector<HTMLElement>(
        '[data-skeleton-region="navigation"]',
      );
      expect(rail).toHaveClass("sticky", "@4xl/page:col-start-2");
      expect(rail!.querySelectorAll("nav a[href^='#']")).toHaveLength(links);
      unmount();
    },
  );

  it.each([
    [<BlueprintCreatePageSkeleton key="blueprint" />, "blueprint-create"],
    [<KeyValueCreatePageSkeleton key="keyvalue" />, "keyvalue-create"],
    [<WebhookCreatePageSkeleton key="webhook" />, "webhook-create"],
  ])(
    "titles the create form above its card, in the form-width frame",
    (component, shape) => {
      const { container, unmount } = render(component);
      const frame = container.querySelector<HTMLElement>(
        `[data-route-skeleton="${shape}"]`,
      );
      expect(frame?.querySelector(".max-w-2xl")).not.toBeNull();
      const title = frame!.querySelector(
        '[data-skeleton-region="page-header"] h1',
      );
      expect(title?.textContent).not.toBe("");
      expect(title?.closest('[data-slot="card"]')).toBeNull();
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

  // w4/446: each service chart section reserves the ready event lane above a
  // plot at the chart's own ratio, not a fixed 180px bar.
  it("previews each service chart's event lane and ratio-sized plot", () => {
    const { container } = render(<ServiceMetricsSkeleton />);
    const plots = [
      ...container.querySelectorAll('[data-slot="skeleton"]'),
    ].filter((el) => (el as HTMLElement).style.aspectRatio !== "");
    expect(plots).toHaveLength(6);
    for (const plot of plots) {
      expect(plot.previousElementSibling).toHaveClass("h-6");
    }
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
      // The type's own create title, in the shared page header.
      expect(
        container.querySelector('[data-skeleton-region="page-header"] h1')
          ?.textContent,
      ).not.toBe("");
    },
  );
});

// w4/546: a cron job's plan cards sit under the run-time billing hint, so the
// pending form draws it too and the cards don't shift when the form loads.
it("previews a cron job's plan billing hint", async () => {
  const { container } = renderServiceCreateSkeleton(
    "/services/new?type=cron_job",
  );
  const picker = await waitFor(() => {
    const region = container.querySelector(
      '[data-skeleton-region="plan-picker"]',
    );
    expect(region).not.toBeNull();
    return region as HTMLElement;
  });
  expect(picker).toHaveTextContent(
    "A cron job is billed per second while a run executes. The minimum charge is $0.70 a month per cron job.",
  );
});

describe("service plan pending groups (w4/m196)", () => {
  // The card grid is each group's last child: a label always precedes it and
  // an SMTP note may (w8/m127).
  function groupCards(container: HTMLElement): number[] {
    return [
      ...container.querySelectorAll(
        '[data-route-skeleton="service-plan"] [data-instance-type-group]',
      ),
    ].map((group) => group.lastElementChild?.children.length ?? 0);
  }

  it.each([
    ["web_service", [1, 6]],
    ["cron_job", [6]],
    ["background_worker", [6]],
    ["private_service", [6]],
    // Undecided: never guess Free eligibility — only the shared paid ladder.
    [null, [6]],
  ])(
    "parent-pending plan frame for %s reserves its ready Free/Paid groups",
    async (serviceType, cards) => {
      const { container } = renderServicePlanParentSkeleton(serviceType);
      await waitFor(() => expect(groupCards(container)).toEqual(cards));
    },
  );
});

// The Runtime and Edge Caching cards reserve the heights measured in Chromium
// at 390/640/768/1024/1280/1440 (en), one per breakpoint where the ready card
// changes: the md sidebar narrows the card until xl, so a single
// mobile/desktop pair mismatched at 768 and 1024.
describe("service settings card heights per breakpoint", () => {
  function regionCard(
    plan: string | null,
    region: "runtime" | "edge-caching",
  ): Element | null {
    const { container } = render(
      <ServiceSettingsSkeleton
        service={{
          type: "web_service",
          repo: "https://github.com/example/app",
          runtime: "node",
          plan,
        }}
      />,
    );
    return container.querySelector(
      `[data-skeleton-region="${region}"] [data-slot="card"]`,
    );
  }

  it("sizes the Runtime card at every breakpoint", () => {
    expect(regionCard("starter", "runtime")).toHaveClass(
      "h-[294px]",
      "sm:h-[210px]",
      "md:h-[230px]",
      "xl:h-[210px]",
    );
  });

  it.each([
    ["starter", "h-[520px]"],
    ["free", "h-[540px]"],
  ])(
    "sizes a %s plan's Edge Caching card at every breakpoint",
    (plan, base) => {
      expect(regionCard(plan, "edge-caching")).toHaveClass(
        base,
        "sm:h-[456px]",
        "md:h-[488px]",
        "xl:h-[436px]",
        "min-[90rem]:h-[420px]",
      );
    },
  );
});
