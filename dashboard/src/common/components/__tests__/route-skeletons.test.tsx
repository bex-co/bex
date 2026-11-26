import { SERVICE_TYPE_CREATE_PATHS } from "@/features/services/lib/create-context";
import { render, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import {
  AccountSettingsPageSkeleton,
  AuthWidgetSkeleton,
  BillingPageSkeleton,
  BlueprintCreatePageSkeleton,
  DatabaseAppsSkeleton,
  DatabaseCreatePageSkeleton,
  DatabaseInfoSkeleton,
  DatabaseMetricsSkeleton,
  DatabasePlanSkeleton,
  DatabaseRecoverySkeleton,
  DatastoreLogsSkeleton,
  EnvGroupDetailContentSkeleton,
  EnvGroupsListPageSkeleton,
  KeyValueCreatePageSkeleton,
  KeyValueInfoSkeleton,
  NotificationsPageSkeleton,
  ProjectOverviewPageSkeleton,
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

// The Settings frame's Team row reads the root route's session (w7/095), and
// several frames here render without a router.
vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({ session: null }),
}));

function regions(container: HTMLElement): string[] {
  return [...container.querySelectorAll<HTMLElement>("[data-skeleton-region]")]
    .map((element) => element.dataset.skeletonRegion)
    .filter((name): name is string => Boolean(name));
}

function renderServiceCreateSkeleton(path: string) {
  const rootRoute = createRootRoute();
  // The type is the path, Render's create URLs (/web/new …).
  const createRouteNodes = Object.values(SERVICE_TYPE_CREATE_PATHS).map(
    (createPath) =>
      createRoute({
        getParentRoute: () => rootRoute,
        path: createPath,
        component: ServiceCreatePageSkeleton,
      }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren(createRouteNodes),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: {},
  });
  return render(<RouterProvider router={router} />);
}

function renderServiceRootSkeleton(
  base: "/services" | "/static",
  serviceType: string | null = null,
) {
  const rootRoute = createRootRoute({ component: Outlet });
  const resourceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: base === "/services" ? "/services/$serviceId" : "/static/$serviceId",
    component: Outlet,
  });
  const indexRoute = createRoute({
    getParentRoute: () => resourceRoute,
    path: "/",
    component: () => (
      <ServiceRouteContentSkeleton base={base} serviceType={serviceType} />
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([resourceRoute.addChildren([indexRoute])]),
    history: createMemoryHistory({ initialEntries: [`${base}/srv-1`] }),
    context: {},
  });
  return render(<RouterProvider router={router} />);
}

function renderServiceTabParentSkeleton(
  serviceType: string | null,
  tab: "compute" | "metrics" = "compute",
) {
  const rootRoute = createRootRoute({ component: Outlet });
  const resourceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => (
      <ServiceRouteContentSkeleton base="/services" serviceType={serviceType} />
    ),
  });
  const computeRoute = createRoute({
    getParentRoute: () => resourceRoute,
    path: `/${tab}`,
    component: () => null,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      resourceRoute.addChildren([computeRoute]),
    ]),
    history: createMemoryHistory({
      initialEntries: [`/services/srv-1/${tab}`],
    }),
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
      <DatabaseCreatePageSkeleton key="database-create" />,
      "database-create",
      [
        "page-header",
        "name",
        "project-environment",
        "physical-names",
        "version",
        "plan-picker",
        "storage",
        "high-availability",
        "public-access",
        "actions",
      ],
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
    // A Postgres database's pages (w3/m641), as the manifest lists them.
    [
      <DatabaseInfoSkeleton key="database-info" />,
      "database-info",
      [
        "metadata",
        "connection",
        "access-control",
        "sql-console",
        "danger-zone",
      ],
    ],
    [
      <DatabaseMetricsSkeleton key="database-metrics" />,
      "database-metrics",
      ["datastore-metrics", "insights"],
    ],
    [
      <DatabasePlanSkeleton key="database-plan" />,
      "database-plan",
      ["plan", "high-availability"],
    ],
    [<DatabaseAppsSkeleton key="database-apps" />, "database-apps", ["apps"]],
    [
      <DatabaseRecoverySkeleton key="database-recovery" />,
      "database-recovery",
      ["recovery"],
    ],
    // A Key Value instance's Info page (w3/m649), as the manifest lists it.
    [
      <KeyValueInfoSkeleton key="keyvalue-info" />,
      "keyvalue-info",
      [
        "metadata",
        "maxmemory-policy",
        "persistence-mode",
        "plan",
        "connection",
        "networking",
        "danger-zone",
      ],
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
    const { container } = renderServiceCreateSkeleton("/web/new");
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

    // The picker's own shape: the search and the accounts button on one row,
    // over the repository list's fixed box (the frame the picker itself shows
    // while its repositories load).
    expect(toolbar).not.toBeNull();
    expect(toolbar).toHaveClass("flex", "flex-wrap");
    expect(toolbar?.children).toHaveLength(2);
    const list = repositories?.querySelector("[data-repo-list]");
    expect(list).toHaveClass("h-52", "sm:h-[17rem]");
    expect(list).toHaveAttribute("aria-busy", "true");
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

  it.each([[<ServiceSettingsSkeleton key="service" />, 13]])(
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
      const rows = nav!.querySelectorAll('[data-slot="skeleton"]');
      expect(rows).toHaveLength(count);
      // A rail row is a link's 36px (its text and py-2), not a 32px pill (w3/104).
      for (const row of rows) expect(row).toHaveClass("@4xl/page:h-9");
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
    // Edit sits in the header's action slot, as on the loaded card.
    expect(
      heading?.querySelector(
        '[data-slot="card-action"] [data-slot="skeleton"]',
      ),
    ).toHaveClass("h-8", "w-14");
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
    expect(docker.className).toContain("h-[1042px]");
    expect(docker.className).toContain("sm:h-[794px]");
    expect(docker.rows).toBe(8);
    const native = buildCard("node");
    expect(native.className).toContain("h-[920px]");
    expect(native.className).toContain("sm:h-[680px]");
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
      [
        "account",
        "appearance",
        "integrations",
        "access",
        "security",
        "danger-zone",
      ].map((name) => section(name).querySelector("h2")?.textContent),
    ).toEqual([
      "Account",
      "Appearance",
      "Integrations",
      "Access credentials",
      "Security",
      "Danger zone",
    ]);
    // The Kratos flow's six cards, in the order Ory draws them, each with Ory's
    // own title reserved.
    expect(frames("account")).toEqual([
      "account-settings-flow-skeleton",
      // Render's Avatar row after the profile (w1/m648).
      "account-avatar-card-skeleton",
    ]);
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
    // Render's Appearance (w3/m528, w3/m531): the card's three rows.
    expect(frames("appearance")).toEqual(["appearance-card-skeleton"]);
    expect(section("appearance")).toHaveTextContent("Dashboard Theme");
    expect(section("appearance")).toHaveTextContent("Log Explorer Theme");
    expect(section("appearance")).toHaveTextContent("High Contrast Mode");
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
    [<AccountSettingsPageSkeleton key="account" />, "account-settings", 6],
    // Billing email joined the Billing page's rail in w6/m627, billing info
    // in w6/m628.
    [<BillingPageSkeleton key="billing" />, "billing", 9],
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

  // Render's create template (create-form.tsx): rows in the standard-width
  // frame, the ready page's action bar pinned in the frame's footer.
  it.each([
    [<BlueprintCreatePageSkeleton key="blueprint" />, "blueprint-create"],
    [<KeyValueCreatePageSkeleton key="keyvalue" />, "keyvalue-create"],
    [<DatabaseCreatePageSkeleton key="database" />, "database-create"],
  ])(
    "lays the %s rows out in the standard frame above the pinned action bar",
    (component, shape) => {
      const { container, unmount } = render(component);
      const frame = container.querySelector<HTMLElement>(
        `[data-route-skeleton="${shape}"]`,
      );
      expect(frame?.querySelector(".max-w-5xl")).not.toBeNull();
      expect(
        frame!.querySelector('[data-skeleton-region="page-header"] h1')
          ?.textContent,
      ).not.toBe("");
      const footer = frame!.querySelector(
        '[data-skeleton-region="page-footer"]',
      );
      expect(footer).toHaveClass("sticky");
      expect(
        footer!.querySelector('[data-skeleton-region="actions"] button'),
      ).toBeDisabled();
      unmount();
    },
  );

  it.each([[<WebhookCreatePageSkeleton key="webhook" />, "webhook-create"]])(
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

  // The ready header is the eyebrow's 16px line over the 32px name row, and
  // the environments row's buttons wrap under the picker at 390px; a 12px
  // eyebrow with an 8px gap and a 20px heading put the card 4px off (w4/715).
  it("reserves the project page's header and environments row at their ready heights", () => {
    const { container } = render(<ProjectOverviewPageSkeleton />);
    const header = container.querySelector<HTMLElement>(
      '[data-skeleton-region="project-header"]',
    )!;
    expect(header).not.toHaveClass("space-y-2");
    const eyebrow = header.firstElementChild!;
    expect(eyebrow).toHaveClass("text-xs", "uppercase", "tracking-wide");
    expect(eyebrow).toHaveTextContent("Project");

    const environments = container.querySelector<HTMLElement>(
      '[data-skeleton-region="environments"]',
    )!;
    expect(environments.querySelector(".h-7 > .h-5")).not.toBeNull();
    expect(environments).toHaveTextContent("New Service");
    expect(environments).toHaveTextContent("New Environment");
  });

  it("uses editor-card geometry for the service environment skeleton", () => {
    const { container } = render(<ServiceEnvironmentSkeleton />);
    expect(container.querySelectorAll('[data-slot="card"]')).toHaveLength(3);
  });

  it.each([
    ["/services", null, "deploys-list"],
    ["/services", "web_service", "deploys-list"],
    // A cron job's bare URL opens Runs, as on Render (w3/m554).
    ["/services", "cron_job", "cron-runs"],
    ["/static", null, "service-events"],
  ] as const)(
    "selects the canonical root destination for %s (%s)",
    async (base, serviceType, expectedShape) => {
      const { container } = renderServiceRootSkeleton(base, serviceType);
      await waitFor(() =>
        expect(
          container.querySelector(`[data-route-skeleton="${expectedShape}"]`),
        ).not.toBeNull(),
      );
    },
  );

  it("uses a full Ory-card silhouette for in-page flow loading", () => {
    const { container } = render(<AuthWidgetSkeleton flow="recovery" />);
    const widget = container.querySelector(
      '[data-skeleton-region="auth-widget"]',
    );
    // Ory's card: square with only its bottom border below `sm`.
    expect(widget).toHaveClass(
      "border-b",
      "p-6",
      "sm:rounded-xl",
      "sm:border",
      "sm:p-8",
    );
    // Title, subtitle, the e-mail field's label and input, and "Continue".
    expect(widget!.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(5);
  });

  // w3/103: a cron job's ready page is the range picker alone, then Network
  // Metrics with Outbound Bandwidth (its plot and event lane) and no controls.
  it("draws a cron job's metrics frame as the range picker and Outbound Bandwidth", () => {
    const { container } = render(
      <ServiceMetricsSkeleton cronJob servesRequests={false} />,
    );
    expect(regions(container)).toEqual(["metrics-filters", "network-metrics"]);
    const region = (name: string) =>
      container.querySelector(`[data-skeleton-region="${name}"]`)!;
    const toolbar = region("metrics-filters").querySelectorAll(
      '[data-slot="skeleton"]',
    );
    expect(toolbar).toHaveLength(1);
    expect(toolbar[0]).toHaveClass("h-8", "w-40");
    const plots = [
      ...region("network-metrics").querySelectorAll('[data-slot="skeleton"]'),
    ].filter((el) => (el as HTMLElement).style.aspectRatio !== "");
    expect(plots).toHaveLength(1);
    expect(plots[0].previousElementSibling).toHaveClass("h-6");
    // Title and subtitle, the section's title, its plot and the month-to-date
    // line: no filters.
    expect(
      region("network-metrics").querySelectorAll('[data-slot="skeleton"]'),
    ).toHaveLength(5);
  });

  // w3/106: MetricsFilters' row is two 160 px selects and a 38 px toggle, each
  // 44 px tall below lg (the touch rule); a 32 px frame there started the
  // cards 12 px above the page's at 390, and the range select was 32 px short.
  it("draws a service's metrics toolbar at the ready row's sizes", () => {
    const { container } = render(<ServiceMetricsSkeleton />);
    const toolbar = container.querySelector(
      '[data-skeleton-region="metrics-filters"]',
    )!;
    expect(toolbar).toHaveClass("max-sm:flex-nowrap", "max-sm:gap-2");
    const [events, range, toggle] = toolbar.querySelectorAll(
      '[data-slot="skeleton"]',
    );
    for (const select of [events, range]) {
      expect(select).toHaveClass("h-8", "w-40", "max-lg:h-11", "max-sm:flex-1");
    }
    expect(toggle).toHaveClass("ml-auto", "h-8", "w-[38px]", "max-lg:size-11");
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

  // The ready page opens on its heading and the Source Code row; the
  // settings and the action bar wait for a source, so the pending frame draws
  // neither — and a static site's picker has no Existing Image tab.
  it.each([
    ["/web/new", "New Web Service", 3],
    ["/static/new", "New Static Site", 2],
    ["/cron/new", "New Cron Job", 3],
  ] as const)(
    "draws %s's heading and Source Code row alone",
    async (path, title, tabs) => {
      const { container } = renderServiceCreateSkeleton(path);

      await waitFor(() =>
        expect(
          container.querySelector('[data-route-skeleton="service-create"]'),
        ).not.toBeNull(),
      );
      expect(regions(container)).toEqual([
        "page-header",
        "service-form",
        "source-picker",
        "source-toolbar",
        "source-repositories",
      ]);
      expect(
        container.querySelector('[data-skeleton-region="page-header"] h1')
          ?.textContent,
      ).toBe(title);
      const source = container.querySelector(
        '[data-skeleton-region="source-picker"]',
      )!;
      expect(source).toHaveTextContent("Source Code");
      // The underline tabs' row: one slot per tab.
      expect(source.querySelector(".border-b")!.children).toHaveLength(tabs);
      expect(
        container.querySelector('[data-skeleton-region="page-footer"]'),
      ).toBeNull();
    },
  );
});

describe("service compute pending frame (w6/m483)", () => {
  function regions(container: HTMLElement): string[] {
    return [
      ...container.querySelectorAll(
        '[data-route-skeleton="service-compute"] [data-skeleton-region]',
      ),
    ].map((region) => region.getAttribute("data-skeleton-region")!);
  }

  const SCALABLE = [
    "compute-plan",
    "autoscaling",
    "manual-scaling",
    "recent-metrics",
  ];

  it.each([
    ["web_service", SCALABLE],
    ["private_service", SCALABLE],
    ["background_worker", SCALABLE],
    // Undecided: reserve the sections every other compute type has.
    [null, SCALABLE],
  ])(
    "parent-pending Compute frame for %s reserves its ready sections",
    async (serviceType, expected) => {
      const { container } = renderServiceTabParentSkeleton(serviceType);
      await waitFor(() => expect(regions(container)).toEqual(expected));
      // The Compute section's title is static copy, drawn as on the ready card.
      expect(
        container.querySelector(
          '[data-skeleton-region="compute-plan"] [data-slot="card-title"]',
        ),
      ).toHaveTextContent("Compute");
    },
  );

  // A cron job has no Compute page: its Compute URL lands on Settings, whose
  // frame the parent's pending state previews instead (w3/m554).
  it("previews Settings for a cron job's Compute URL", async () => {
    const { container } = renderServiceTabParentSkeleton("cron_job");
    await waitFor(() =>
      expect(
        container.querySelector('[data-route-skeleton="service-settings"]'),
      ).not.toBeNull(),
    );
    expect(regions(container)).toEqual([]);
  });
});

describe("service metrics pending frame (w3/103)", () => {
  // w3/103: the parent's pending state previews a cron job's Metrics page as
  // the range picker and Outbound Bandwidth, a worker's with its event
  // controls and Application card.
  it.each([
    ["cron_job", ["metrics-filters", "network-metrics"], 1],
    [
      "background_worker",
      ["metrics-filters", "application-metrics", "network-metrics"],
      3,
    ],
  ])(
    "previews a %s's Metrics page",
    async (serviceType, expected, toolbarSlots) => {
      const { container } = renderServiceTabParentSkeleton(
        serviceType,
        "metrics",
      );
      await waitFor(() =>
        expect(
          container.querySelector('[data-route-skeleton="service-metrics"]'),
        ).not.toBeNull(),
      );
      expect(regions(container)).toEqual(expected);
      expect(
        container.querySelectorAll(
          '[data-skeleton-region="metrics-filters"] [data-slot="skeleton"]',
        ),
      ).toHaveLength(toolbarSlots);
    },
  );
});

// The Runtime and Edge Caching cards reserve the heights measured in Chromium
// at 390/640/768/1024/1280/1440 (en), one per breakpoint where the ready card
// changes: the md sidebar narrows the card at 768, and from lg its controls
// drop to their desktop height (w3/104).
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
      "h-[318px]",
      "sm:h-[222px]",
      "md:h-[242px]",
      "lg:h-[210px]",
    );
  });

  it.each([
    ["starter", "h-[532px]"],
    ["free", "h-[552px]"],
  ])(
    "sizes a %s plan's Edge Caching card at every breakpoint",
    (plan, base) => {
      const card = regionCard(plan, "edge-caching");
      expect(card).toHaveClass(
        base,
        "sm:h-[468px]",
        "md:h-[500px]",
        "lg:h-[436px]",
      );
      // No wider override: at 1440 the card is lg's 436px.
      expect(card?.className).not.toContain("min-[90rem]");
    },
  );
});

// w3/104: the heights the ready cards measured in Chromium at 390 and 1440
// for each service shape whose cards differ, so a shape cannot fall back to
// another shape's card.
describe("service settings card heights per service shape", () => {
  function card(
    service: ComponentProps<typeof ServiceSettingsSkeleton>["service"],
    region: string,
    index = 0,
  ): Element | undefined {
    const { container } = render(<ServiceSettingsSkeleton service={service} />);
    return [
      ...container.querySelectorAll(
        `[data-skeleton-region="${region}"] > [data-slot="card"]`,
      ),
    ][index];
  }
  const image = { repo: null, runtime: "image" };
  const native = {
    repo: "https://github.com/example/app",
    runtime: "node",
    builder: "native",
  };
  const docker = {
    repo: "https://github.com/example/app",
    runtime: "docker",
    builder: "dockerfile",
  };

  it.each([
    [
      "a running web service",
      { type: "web_service" },
      "h-[202px]",
      "sm:h-[154px]",
    ],
    [
      "a running static site",
      { type: "static_site" },
      "h-[202px]",
      "sm:h-[154px]",
    ],
    // A cron job's suspend copy runs a line longer at desktop than a worker's.
    ["a running cron job", { type: "cron_job" }, "h-[182px]", "sm:h-[154px]"],
    [
      "a running worker",
      { type: "background_worker" },
      "h-[182px]",
      "sm:h-[134px]",
    ],
    [
      "a running private service",
      { type: "private_service" },
      "h-[182px]",
      "sm:h-[134px]",
    ],
    [
      "a suspended web service",
      { type: "web_service", suspended: true },
      "h-[162px]",
      "sm:h-[134px]",
    ],
    [
      "a suspended cron job",
      { type: "cron_job", suspended: true },
      "h-[142px]",
      "sm:h-[134px]",
    ],
  ])("sizes %s's Suspend card", (_, shape, narrow, wide) => {
    expect(card({ ...image, ...shape }, "suspend")).toHaveClass(narrow, wide);
  });

  it("sizes an image source's card below a repo's on a phone", () => {
    expect(card({ type: "web_service", ...image }, "source")).toHaveClass(
      "h-[168px]",
      "sm:h-[148px]",
    );
    expect(card({ type: "web_service", ...native }, "source")).toHaveClass(
      "h-[228px]",
      "sm:h-[148px]",
    );
  });

  it.each([
    ["starter", undefined, "h-[682px]", "sm:h-[530px]"],
    // A free plan says Pre-Deploy needs a paid plan, unless one is set.
    ["free", undefined, "h-[730px]", "sm:h-[558px]"],
    ["free", "npm run migrate", "h-[682px]", "sm:h-[530px]"],
  ])(
    "sizes a %s image service's Deploy card (pre-deploy %s)",
    (plan, preDeployCommand, narrow, wide) => {
      expect(
        card(
          { type: "web_service", ...image, plan, preDeployCommand },
          "deploy",
        ),
      ).toHaveClass(narrow, wide);
    },
  );

  it.each([
    ["native", native, "starter", "h-[820px]", "sm:h-[642px]"],
    ["native", native, "free", "h-[868px]", "sm:h-[670px]"],
    ["Dockerfile", docker, "starter", "h-[822px]", "sm:h-[644px]"],
    ["Dockerfile", docker, "free", "h-[870px]", "sm:h-[672px]"],
  ])(
    "sizes a %s build's Deploy card on a %s plan",
    (_, source, plan, narrow, wide) => {
      expect(
        card({ type: "web_service", ...source, plan }, "build", 1),
      ).toHaveClass(narrow, wide);
    },
  );

  it.each([
    ["native", native, "h-[382px]", "sm:h-[326px]"],
    ["Dockerfile", docker, "h-[384px]", "sm:h-[328px]"],
    ["image", image, "h-[384px]", "sm:h-[328px]"],
  ])("sizes a %s cron job's Deploy card", (_, source, narrow, wide) => {
    expect(card({ type: "cron_job", ...source }, "deploy")).toHaveClass(
      narrow,
      wide,
    );
  });

  it("sizes a cron job's lone Build card, the same for every build", () => {
    for (const source of [native, docker]) {
      expect(card({ type: "cron_job", ...source }, "build")).toHaveClass(
        "h-[898px]",
        "sm:h-[680px]",
      );
    }
  });
});
