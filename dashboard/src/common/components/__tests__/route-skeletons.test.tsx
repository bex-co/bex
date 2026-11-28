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
import { en } from "@/i18n";
import {
  AccountSettingsPageSkeleton,
  AuthWidgetSkeleton,
  BillingPageSkeleton,
  BlueprintCreatePageSkeleton,
  BlueprintResourcesSkeleton,
  BlueprintSettingsSkeleton,
  BlueprintSyncsSkeleton,
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
  ServiceShellSkeleton,
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
  tab: "compute" | "metrics" | "shell" = "compute",
  props: Partial<ComponentProps<typeof ServiceRouteContentSkeleton>> = {},
) {
  const rootRoute = createRootRoute({ component: Outlet });
  const resourceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => (
      <ServiceRouteContentSkeleton
        base="/services"
        serviceType={serviceType}
        {...props}
      />
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
    // A Blueprint's pages (w3/m677), as the manifest lists them.
    [
      <BlueprintResourcesSkeleton key="blueprint-resources" />,
      "blueprint-resources",
      ["resources"],
    ],
    [
      <BlueprintSyncsSkeleton key="blueprint-syncs" />,
      "blueprint-syncs",
      ["sync-history"],
    ],
    [
      <BlueprintSettingsSkeleton key="blueprint-settings" />,
      "blueprint-settings",
      ["settings", "details", "manifest", "validation", "disconnect"],
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
    // Render's header carries + Add environment (w1/m682).
    expect(header).toHaveTextContent("Add environment");

    const environments = container.querySelector<HTMLElement>(
      '[data-skeleton-region="environments"]',
    )!;
    // One section: its 28px name line over the switcher row and search.
    expect(environments.querySelector(".h-7 > .h-5")).not.toBeNull();
    expect(environments).toHaveTextContent("New Service");
    expect(environments).toHaveTextContent("Manage resources");
    expect(environments).toHaveTextContent("Env Groups (0)");
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

// w3/124: the Metrics frame drew each card as a title over a subtitle with
// three 36 px boxes beside it, and each section as a 28 px row with 8 px gaps
// around the event lane, so the Application card was 76 px taller than the
// ready one at 1440 and 48 px at 390, and the Network card took the same
// shape. Each card now lays out the ready card's own controls and titles.
describe("service metrics cards on their ready geometry (w3/124)", () => {
  const region = (container: HTMLElement, name: string) =>
    container.querySelector(`[data-skeleton-region="${name}"]`)!;
  const words = (element: Element) =>
    [...element.querySelectorAll("span.invisible")].map(
      (span) => span.textContent,
    );
  const sections = (card: Element) => [
    ...card.querySelector('[data-slot="card-content"]')!.children,
  ];
  // A section is MetricSection's title row, the event lane and the plot.
  function expectChartSection(section: Element, title: string) {
    const [row, lane, plot] = section.children;
    expect(row).toHaveClass("mb-2", "flex", "flex-wrap", "gap-y-1");
    expect(row.firstElementChild).toHaveClass("relative", "text-sm");
    expect(row.querySelector("span.invisible")).toHaveTextContent(title);
    expect(lane).toHaveClass("h-6");
    expect((plot as HTMLElement).style.aspectRatio).not.toBe("");
  }

  it("draws the Application card's title alone and its controls under it", () => {
    const { container } = render(<ServiceMetricsSkeleton />);
    const card = region(container, "application-metrics");
    const header = card.querySelector('[data-slot="card-header"]')!;
    expect(header).toHaveClass("flex-row", "flex-wrap", "gap-3");
    const [title, controls] = header.children;
    expect(title).toHaveClass("h-4");
    const [filter, aggregate, mode] = controls.children;
    expect(filter).toHaveClass(
      "h-9",
      "w-36",
      "max-lg:h-11",
      "pointer-coarse:h-11",
    );
    expect(words(aggregate)).toEqual(["Raw", "Min", "Max", "Avg"]);
    expect(words(mode)).toEqual(["Percentage", "Total"]);
    // The segmented controls grow to 50 px where each tab takes 44.
    expect(aggregate).toHaveClass("h-9", "p-[3px]", "max-lg:h-auto");
    expect(aggregate.querySelector("span.invisible")).toHaveClass(
      "px-2",
      "max-lg:min-h-11",
      "pointer-coarse:min-h-11",
    );
    const charts = sections(card);
    expect(charts).toHaveLength(3);
    ["Memory", "CPU", "Total Instances"].forEach((name, index) =>
      expectChartSection(charts[index], name),
    );
  });

  it("leaves the instance filter out while the service runs no instances", () => {
    const { container } = render(
      <ServiceMetricsSkeleton runsInstances={false} />,
    );
    const controls = region(container, "application-metrics").querySelector(
      '[data-slot="card-header"] > :nth-child(2)',
    )!;
    expect(controls.children).toHaveLength(2);
    expect(words(controls)).toEqual([
      "Raw",
      "Min",
      "Max",
      "Avg",
      "Percentage",
      "Total",
    ]);
  });

  it("draws the Network card's filters, request charts and month-to-date line", () => {
    const { container } = render(<ServiceMetricsSkeleton />);
    const card = region(container, "network-metrics");
    const [heading, filters] = card.querySelector(
      '[data-slot="card-header"]',
    )!.children;
    expect(words(heading)).toEqual(["Aggregated across all instances"]);
    const [status, host, path] = filters.children;
    expect(words(status)).toEqual(["Status Code", "All"]);
    expect(words(host)).toEqual(["Host", "All"]);
    for (const select of [status, host]) {
      expect(select).toHaveClass(
        "h-8",
        "min-w-36",
        "max-lg:min-h-11",
        "max-sm:flex-1",
        "max-[24rem]:basis-full",
      );
    }
    expect(path).toHaveClass("h-8", "w-40", "max-sm:basis-full", "max-lg:h-11");
    const [requests, latency, bandwidth, monthToDate] = sections(card);
    expectChartSection(requests, "Total Requests");
    expect(words(requests)).toEqual([
      "Total Requests",
      "Group by",
      "All requests",
    ]);
    expectChartSection(latency, "Response Times");
    expect(words(latency)).toEqual(["Response Times", "Percentile", "p90"]);
    expectChartSection(bandwidth, "Outbound Bandwidth");
    expect(monthToDate).toHaveClass("h-5");
  });

  it("draws a worker's Network card as Outbound Bandwidth and the month-to-date line", () => {
    const { container } = render(
      <ServiceMetricsSkeleton servesRequests={false} />,
    );
    const card = region(container, "network-metrics");
    expect(
      card.querySelector('[data-slot="card-header"]')!.children,
    ).toHaveLength(1);
    const [bandwidth, monthToDate] = sections(card);
    expectChartSection(bandwidth, "Outbound Bandwidth");
    expect(monthToDate).toHaveClass("h-5");
  });

  it("previews a suspended service's Metrics page from the parent with no instance filter", async () => {
    const { container } = renderServiceTabParentSkeleton(
      "web_service",
      "metrics",
      { runsInstances: false },
    );
    await waitFor(() =>
      expect(
        container.querySelector('[data-skeleton-region="application-metrics"]'),
      ).not.toBeNull(),
    );
    expect(
      container.querySelector(
        '[data-skeleton-region="application-metrics"] [data-slot="card-header"] > :nth-child(2)',
      )!.children,
    ).toHaveLength(2);
  });
});

// w3/124: the Shell frame drew the Web Shell and a full-width SSH card for
// every service, where a Free, suspended or shell-less service lands on one
// "Shell access isn't available" card (174 px at 1440), and the ready SSH
// card keeps a reading measure.
describe("service shell frame (w3/124)", () => {
  const region = (container: HTMLElement, name: string) =>
    container.querySelector(`[data-skeleton-region="${name}"]`)!;

  it("previews the Web Shell and the SSH command on the ready cards", () => {
    const { container } = render(
      <ServiceShellSkeleton access={{ eligible: true }} />,
    );
    expect(regions(container)).toEqual([
      "page-header",
      "web-terminal",
      "ssh-connection",
    ]);
    // The description is the page's own line, wrapping where it does.
    const description = region(container, "page-header");
    expect(description).toHaveClass("relative", "max-w-prose", "-mt-3");
    expect(description.querySelector("span.invisible")).toHaveTextContent(
      en["services.shellDescription"],
    );
    const terminal = region(container, "web-terminal");
    expect(
      terminal.querySelector('[data-slot="card-description"] span.invisible'),
    ).toHaveTextContent(en["services.shellWebDescription"]);
    expect(terminal.querySelector(".w-\\[220px\\]")).toHaveClass(
      "h-8",
      "max-lg:h-11",
    );
    const ssh = region(container, "ssh-connection");
    expect(ssh.querySelector('[data-slot="card"]')).toHaveClass("max-w-3xl");
    expect(ssh.textContent).toContain(en["services.shellSessionLifecycle"]);
    expect(ssh.textContent).toContain(en["services.shellManageKeys"]);
  });

  it("draws the terminal while the service's access is unknown", () => {
    const { container } = render(<ServiceShellSkeleton />);
    expect(regions(container)).toContain("web-terminal");
  });

  it.each([
    [
      "a Free service",
      {
        reason: "services.sshUnavailableFree",
        label: "services.sshRemedyChangePlan",
      },
      en["services.sshUnavailableFree"],
      en["services.sshRemedyChangePlan"],
    ],
    [
      "a service the gateway gives no address",
      { reason: "services.sshUnavailableHint", label: null },
      en["services.shellUnavailableBody"],
      en["services.shellManageKeys"],
    ],
  ] as const)(
    "draws %s as the one unavailable card",
    (_, remedy, description, button) => {
      const { container } = render(
        <ServiceShellSkeleton
          access={{
            eligible: false,
            remedy: {
              reason: remedy.reason,
              action: remedy.label
                ? { label: remedy.label, path: "plan" }
                : null,
            },
          }}
        />,
      );
      expect(regions(container)).toEqual(["page-header", "ssh-connection"]);
      const card = region(container, "ssh-connection").querySelector(
        '[data-slot="card"]',
      )!;
      expect(card).toHaveClass("max-w-3xl");
      expect(
        card.querySelector('[data-slot="card-description"] span.invisible'),
      ).toHaveTextContent(description);
      // The button grows with the touch rule, as the ready link does.
      const action = card.querySelector("[data-touch-spacer]")!;
      expect(action).toHaveTextContent(button);
    },
  );

  it("previews the unavailable card from the parent's cached access", async () => {
    const { container } = renderServiceTabParentSkeleton(
      "web_service",
      "shell",
      {
        shellAccess: {
          eligible: false,
          remedy: { reason: "services.sshUnavailableHint", action: null },
        },
      },
    );
    await waitFor(() =>
      expect(
        container.querySelector('[data-route-skeleton="service-shell"]'),
      ).not.toBeNull(),
    );
    expect(regions(container)).toEqual(["page-header", "ssh-connection"]);
  });
});

// w3/121: a held Database read drew the Recovery card 76 px short at 390,
// because its header and scope line were bars of fixed height where the ready
// copy wraps. The frame now lays out the ready copy invisibly under its bars.
describe("database recovery pending frame (w3/121)", () => {
  function reservedText(element: Element | null | undefined) {
    const copy = element?.querySelector(":scope > span.invisible");
    expect(element).toHaveClass("relative");
    expect(
      element?.querySelector(':scope > [data-slot="skeleton"]'),
    ).toHaveClass("absolute", "inset-0");
    return copy?.textContent;
  }

  it("lays out the ready title and description, so the header wraps as the ready one", () => {
    const { container } = render(
      <DatabaseRecoverySkeleton databaseId="dpg-c185th5c2rvvnhbfiltg" />,
    );
    const header = container.querySelector('[data-slot="card-header"]');
    expect(reservedText(header?.children[0])).toBe("Recovery");
    expect(reservedText(header?.children[1])).toBe(
      "Point-in-time recovery and backups. Restore always creates a new database — this one is never modified.",
    );
    // No second heading inside the frame: the page's h2 names it.
    expect(header?.querySelector("h2")).toBeNull();
  });

  it("names the database a create names none for in the scope line before the read lands", () => {
    const { container } = render(
      <DatabaseRecoverySkeleton databaseId="dpg-c185th5c2rvvnhbfiltg" />,
    );
    const skeleton = container.querySelector(
      '[data-testid="recovery-skeleton"]',
    );
    expect(skeleton?.textContent).toContain(
      "An export contains the database dpg_c185th5c2rvvnhbfiltg only.",
    );
    // The actions and the list headings are the ready copy too, and the one
    // backup row is a single line at every width.
    expect(skeleton?.textContent).toContain("Restore to new instance");
    expect(skeleton?.textContent).toContain("Create export");
    expect(skeleton?.textContent).toContain("Backups");
    expect(skeleton?.textContent).toContain("No exports yet.");
    const row = container.querySelector(
      '[data-testid="recovery-skeleton-backup"]',
    );
    expect(row).toHaveClass("h-10");
    expect(row?.className).not.toMatch(/(^|\s)h-\[/);
  });

  it("names a known database's own name, and leaves the line out where it has none", () => {
    const { container, rerender } = render(
      <DatabaseRecoverySkeleton
        database={{ backupsEnabled: true, databaseName: "orders" }}
        databaseId="dpg-c185th5c2rvvnhbfiltg"
      />,
    );
    const text = () =>
      container.querySelector('[data-testid="recovery-skeleton"]')?.textContent;
    expect(text()).toContain("An export contains the database orders only.");
    rerender(
      <DatabaseRecoverySkeleton
        database={{ backupsEnabled: true, databaseName: null }}
        databaseId="dpg-c185th5c2rvvnhbfiltg"
      />,
    );
    expect(text()).not.toContain("An export contains");
  });

  it("lays out the disabled line's copy for a database without backups", () => {
    const { container } = render(
      <DatabaseRecoverySkeleton
        database={{ backupsEnabled: false, databaseName: "orders" }}
      />,
    );
    const line = container.querySelector(
      '[data-testid="recovery-skeleton"] > p',
    );
    expect(reservedText(line)).toBe(
      "Backups aren't enabled for this plan, so recovery isn't available. Upgrade to a plan with backups to enable point-in-time recovery.",
    );
  });
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

type SettingsService = ComponentProps<
  typeof ServiceSettingsSkeleton
>["service"];

/** The index-th card a Settings skeleton draws for a section. */
function settingsCard(
  service: SettingsService,
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
const imageSource = { repo: null, runtime: "image" };
const nativeSource = {
  repo: "https://github.com/example/app",
  runtime: "node",
  builder: "native",
};
const dockerSource = {
  repo: "https://github.com/example/app",
  runtime: "docker",
  builder: "dockerfile",
};

// w3/104: the heights the ready cards measured in Chromium at 390 and 1440
// for each service shape whose cards differ, so a shape cannot fall back to
// another shape's card.
describe("service settings card heights per service shape", () => {
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
    expect(settingsCard({ ...imageSource, ...shape }, "suspend")).toHaveClass(
      narrow,
      wide,
    );
  });

  it("sizes an image source's card below a repo's on a phone", () => {
    expect(
      settingsCard({ type: "web_service", ...imageSource }, "source"),
    ).toHaveClass("h-[168px]", "sm:h-[148px]");
    expect(
      settingsCard({ type: "web_service", ...nativeSource }, "source"),
    ).toHaveClass("h-[228px]", "sm:h-[148px]");
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
        settingsCard(
          { type: "web_service", ...imageSource, plan, preDeployCommand },
          "deploy",
        ),
      ).toHaveClass(narrow, wide);
    },
  );

  it.each([
    ["native", nativeSource, "starter", "h-[820px]", "sm:h-[642px]"],
    ["native", nativeSource, "free", "h-[868px]", "sm:h-[670px]"],
    ["Dockerfile", dockerSource, "starter", "h-[822px]", "sm:h-[644px]"],
    ["Dockerfile", dockerSource, "free", "h-[870px]", "sm:h-[672px]"],
  ])(
    "sizes a %s build's Deploy card on a %s plan",
    (_, source, plan, narrow, wide) => {
      expect(
        settingsCard({ type: "web_service", ...source, plan }, "build", 1),
      ).toHaveClass(narrow, wide);
    },
  );

  it.each([
    ["native", nativeSource, "h-[382px]", "sm:h-[326px]"],
    ["Dockerfile", dockerSource, "h-[384px]", "sm:h-[328px]"],
    ["image", imageSource, "h-[384px]", "sm:h-[328px]"],
  ])("sizes a %s cron job's Deploy card", (_, source, narrow, wide) => {
    expect(settingsCard({ type: "cron_job", ...source }, "deploy")).toHaveClass(
      narrow,
      wide,
    );
  });

  it("sizes a cron job's lone Build card, the same for every build", () => {
    for (const source of [nativeSource, dockerSource]) {
      expect(
        settingsCard({ type: "cron_job", ...source }, "build"),
      ).toHaveClass("h-[898px]", "sm:h-[680px]");
    }
  });
});

// w3/123: from lg a coarse pointer keeps the touch region's 44px controls, and
// a card whose hint wraps a line more in the narrower desktop column (720px at
// 1024, 696px at 1280) steps up below the width it wraps at. Measured in
// Chromium with and without `hasTouch` at 1024, 1280 and 1440 for the shapes
// w3/104 measured: every pending card then equals its ready card.
describe("service settings card heights with a coarse pointer", () => {
  const web = { type: "web_service", plan: "starter", ...nativeSource };
  const staticSite = { type: "static_site", ...nativeSource };

  it.each<[string, SettingsService, string, number, string[]]>([
    // General: each row's control, and Region's hint below a 738px column.
    [
      "a web service's General",
      web,
      "general",
      0,
      ["sm:h-[434px]", "lg:pointer-coarse:h-[450px]"],
    ],
    [
      "a web service's General with a region",
      { ...web, region: "fsn1" },
      "general",
      0,
      [
        "sm:h-[546px]",
        "lg:pointer-coarse:h-[570px]",
        "lg:@max-[738px]:h-[566px]",
        "lg:@max-[738px]:pointer-coarse:h-[590px]",
      ],
    ],
    [
      "a free web service's General",
      { ...web, plan: "free" },
      "general",
      0,
      ["sm:h-[474px]", "lg:pointer-coarse:h-[490px]"],
    ],
    [
      "a free web service's General with a region",
      { ...web, plan: "free", region: "fsn1" },
      "general",
      0,
      [
        "sm:h-[586px]",
        "lg:pointer-coarse:h-[610px]",
        "lg:@max-[738px]:h-[606px]",
        "lg:@max-[738px]:pointer-coarse:h-[630px]",
      ],
    ],
    [
      "a worker's General",
      { type: "background_worker", ...imageSource },
      "general",
      0,
      ["sm:h-[366px]", "lg:pointer-coarse:h-[382px]"],
    ],
    [
      "a worker's General with a region",
      { type: "background_worker", ...nativeSource, region: "fsn1" },
      "general",
      0,
      [
        "sm:h-[478px]",
        "lg:pointer-coarse:h-[502px]",
        "lg:@max-[738px]:h-[498px]",
        "lg:@max-[738px]:pointer-coarse:h-[522px]",
      ],
    ],
    [
      "a cron job's General",
      { type: "cron_job", ...imageSource },
      "general",
      0,
      ["sm:h-[186px]", "lg:pointer-coarse:h-[194px]"],
    ],
    [
      "a cron job's General with a region",
      { type: "cron_job", ...imageSource, region: "fsn1" },
      "general",
      0,
      [
        "sm:h-[298px]",
        "lg:pointer-coarse:h-[314px]",
        "lg:@max-[738px]:h-[318px]",
        "lg:@max-[738px]:pointer-coarse:h-[334px]",
      ],
    ],
    [
      "a static site's General",
      staticSite,
      "general",
      0,
      ["sm:h-[186px]", "lg:pointer-coarse:h-[194px]"],
    ],
    // Deploy and Build, with each Build card's Root Directory hint below a
    // 750px column.
    [
      "an image service's Deploy",
      { type: "web_service", plan: "starter", ...imageSource },
      "deploy",
      0,
      ["sm:h-[530px]", "lg:pointer-coarse:h-[562px]"],
    ],
    [
      "a free image service's Deploy",
      { type: "web_service", plan: "free", ...imageSource },
      "deploy",
      0,
      ["sm:h-[558px]", "lg:pointer-coarse:h-[590px]"],
    ],
    [
      "a native cron job's Deploy",
      { type: "cron_job", ...nativeSource },
      "deploy",
      0,
      ["sm:h-[326px]", "lg:pointer-coarse:h-[342px]"],
    ],
    [
      "an image cron job's Deploy",
      { type: "cron_job", ...imageSource },
      "deploy",
      0,
      ["sm:h-[328px]", "lg:pointer-coarse:h-[344px]"],
    ],
    [
      "a cron job's Build",
      { type: "cron_job", ...dockerSource },
      "build",
      0,
      [
        "sm:h-[680px]",
        "lg:pointer-coarse:h-[726px]",
        "lg:@max-[750px]:h-[700px]",
        "lg:@max-[750px]:pointer-coarse:h-[746px]",
      ],
    ],
    [
      "a native build's Build",
      web,
      "build",
      0,
      [
        "sm:h-[680px]",
        "lg:pointer-coarse:h-[728px]",
        "lg:@max-[750px]:h-[700px]",
        "lg:@max-[750px]:pointer-coarse:h-[748px]",
      ],
    ],
    [
      "a native build's Deploy",
      web,
      "build",
      1,
      ["sm:h-[642px]", "lg:pointer-coarse:h-[680px]"],
    ],
    [
      "a free native build's Deploy",
      { ...web, plan: "free" },
      "build",
      1,
      ["sm:h-[670px]", "lg:pointer-coarse:h-[708px]"],
    ],
    [
      "a Dockerfile build's Build",
      { ...web, ...dockerSource },
      "build",
      0,
      [
        "sm:h-[794px]",
        "lg:pointer-coarse:h-[850px]",
        "lg:@max-[750px]:h-[814px]",
        "lg:@max-[750px]:pointer-coarse:h-[870px]",
      ],
    ],
    [
      "a Dockerfile build's Deploy",
      { ...web, ...dockerSource },
      "build",
      1,
      ["sm:h-[644px]", "lg:pointer-coarse:h-[682px]"],
    ],
    [
      "a free Dockerfile build's Deploy",
      { ...web, ...dockerSource, plan: "free" },
      "build",
      1,
      ["sm:h-[672px]", "lg:pointer-coarse:h-[710px]"],
    ],
    [
      "a static site's Deploy",
      staticSite,
      "build",
      1,
      ["sm:h-[416px]", "lg:pointer-coarse:h-[438px]"],
    ],
    [
      "a static site's publish directory card",
      staticSite,
      "static-site",
      0,
      [
        "sm:h-[238px]",
        "lg:pointer-coarse:h-[246px]",
        "lg:@max-[722px]:h-[258px]",
        "lg:@max-[722px]:pointer-coarse:h-[266px]",
      ],
    ],
    [
      "Runtime",
      web,
      "runtime",
      0,
      ["lg:h-[210px]", "lg:pointer-coarse:h-[222px]"],
    ],
    [
      "the Registry credential",
      { type: "background_worker", ...imageSource },
      "registry-credential",
      0,
      ["sm:h-[256px]", "lg:pointer-coarse:h-[272px]"],
    ],
    [
      "Notifications",
      web,
      "notifications",
      0,
      ["sm:h-[208px]", "lg:pointer-coarse:h-[214px]"],
    ],
    ["Port", web, "port", 0, ["sm:h-[226px]", "lg:pointer-coarse:h-[234px]"]],
    [
      "Health Checks",
      web,
      "health-checks",
      0,
      ["sm:h-[246px]", "lg:pointer-coarse:h-[254px]"],
    ],
    [
      "Edge Caching",
      web,
      "edge-caching",
      0,
      ["lg:h-[436px]", "lg:pointer-coarse:h-[448px]"],
    ],
    [
      "Maintenance",
      web,
      "maintenance",
      0,
      ["sm:h-[286px]", "lg:pointer-coarse:h-[294px]"],
    ],
    [
      "a cron job's Deploy Hook",
      { type: "cron_job", ...imageSource },
      "deploy-hook",
      0,
      ["sm:h-[222px]", "lg:pointer-coarse:h-[238px]"],
    ],
    // Suspend and Danger Zone: their button.
    [
      "a running web service's Suspend",
      web,
      "suspend",
      0,
      ["sm:h-[154px]", "lg:pointer-coarse:h-[162px]"],
    ],
    [
      "a running cron job's Suspend",
      { type: "cron_job", ...imageSource },
      "suspend",
      0,
      ["sm:h-[154px]", "lg:pointer-coarse:h-[162px]"],
    ],
    [
      "a running worker's Suspend",
      { type: "background_worker", ...imageSource },
      "suspend",
      0,
      ["sm:h-[134px]", "lg:pointer-coarse:h-[142px]"],
    ],
    [
      "a suspended web service's Suspend",
      { ...web, suspended: true },
      "suspend",
      0,
      ["sm:h-[134px]", "lg:pointer-coarse:h-[142px]"],
    ],
    [
      "a suspended cron job's Suspend",
      { type: "cron_job", ...imageSource, suspended: true },
      "suspend",
      0,
      ["sm:h-[134px]", "lg:pointer-coarse:h-[142px]"],
    ],
    [
      "the Danger Zone",
      web,
      "danger-zone",
      0,
      ["sm:h-[134px]", "lg:pointer-coarse:h-[142px]"],
    ],
  ])(
    "reserves the touch height of %s",
    (_, service, region, index, classes) => {
      expect(settingsCard(service, region, index)).toHaveClass(...classes);
    },
  );

  it("leaves the Domains card's height alone: nothing in it grows", () => {
    expect(settingsCard(web, "domains")?.className).not.toContain(
      "pointer-coarse:",
    );
  });

  // A phone with a coarse pointer already has the touch-size controls in its
  // own height, and a desktop column is only narrower from lg: an ungated
  // coarse or column step would override the phone's height.
  it.each<[string, SettingsService]>([
    ["web service", { ...web, region: "fsn1" }],
    ["Dockerfile web service", { ...web, ...dockerSource, plan: "free" }],
    [
      "image web service",
      { type: "web_service", plan: "free", ...imageSource },
    ],
    ["worker", { type: "background_worker", ...imageSource, region: "fsn1" }],
    ["cron job", { type: "cron_job", ...dockerSource, region: "fsn1" }],
    ["static site", staticSite],
  ])("gates every coarse and column step of a %s at lg", (_, service) => {
    const { container } = render(<ServiceSettingsSkeleton service={service} />);
    const steps = [
      ...container.querySelectorAll(
        '[data-skeleton-region] > [data-slot="card"]',
      ),
    ].flatMap((card) =>
      [...card.classList].filter(
        (name) => name.includes("pointer-coarse:") || name.includes("@max-"),
      ),
    );
    expect(steps.length).toBeGreaterThan(0);
    for (const step of steps) expect(step).toMatch(/^lg:/);
  });
});
