import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderToString } from "react-dom/server";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DashboardBreadcrumbs } from "../dashboard-breadcrumbs";
import { GlobalSearch } from "../global-search";
import { HelpMenu } from "../dashboard-header";
import type { ServiceView } from "@/features/services/types";

const mocks = vi.hoisted(() => ({
  service: {
    id: "srv-api",
    name: "storefront-api",
    type: "web_service",
  },
  services: [
    { id: "srv-api", name: "storefront-api", type: "web_service" },
    { id: "srv-worker", name: "queue-worker", type: "background_worker" },
    { id: "srv-billing", name: "billing-api", type: "web_service" },
  ],
  projects: [
    {
      id: "prj-storefront",
      name: "Storefront",
      ownerId: "tea-one",
      serviceIds: ["srv-api", "srv-worker"],
      databaseIds: [],
      keyValueIds: [],
    },
    {
      id: "prj-billing",
      name: "Billing",
      ownerId: "tea-one",
      serviceIds: ["srv-billing"],
      databaseIds: [],
      keyValueIds: [],
    },
  ],
  environments: [
    {
      id: "env-production",
      projectId: "prj-storefront",
      name: "Production",
      ownerId: "tea-one",
      createdAt: null,
      serviceIds: ["srv-api"],
      databaseIds: [],
      keyValueIds: [],
      envGroupIds: [],
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowListEntries: [],
    },
  ],
}));

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: mocks.service as ServiceView,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: mocks.services as ServiceView[],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: mocks.projects,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: mocks.environments,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));

vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({
    databases: [{ id: "dpg-main", name: "primary-db" }],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({
    keyValues: [{ id: "red-cache", name: "session-cache" }],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [{ id: "evg-shared", name: "Shared secrets" }],
    loading: false,
    error: undefined,
  }),
}));

function buildRouter(pathname: string, component: () => React.ReactNode) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const index = createRoute({
    getParentRoute: () => root,
    path: "/",
    component,
  });
  const service = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/$",
    component,
  });
  const project = createRoute({
    getParentRoute: () => root,
    path: "/project/$projectId",
    component: () => null,
  });
  const database = createRoute({
    getParentRoute: () => root,
    path: "/databases/$databaseId",
    component: () => null,
  });
  const keyValue = createRoute({
    getParentRoute: () => root,
    path: "/keyvalue/$keyValueId",
    component: () => null,
  });
  const envGroup = createRoute({
    getParentRoute: () => root,
    path: "/env-groups/$groupId",
    component: () => null,
  });
  return createRouter({
    routeTree: root.addChildren([
      index,
      service,
      project,
      database,
      keyValue,
      envGroup,
    ]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: {} as never,
  });
}

beforeEach(() => {
  mocks.service.id = "srv-api";
  mocks.service.name = "storefront-api";
});

describe("dashboard topbar navigation", () => {
  it("links to the bex docs and CLI guide", async () => {
    const user = userEvent.setup();
    render(<HelpMenu />);

    await user.click(
      screen.getByRole("button", { name: "Help and resources" }),
    );

    expect(
      await screen.findByRole("menuitem", { name: "Documentation" }),
    ).toHaveAttribute("href", "https://bex.co/docs");
    expect(screen.getByRole("menuitem", { name: "CLI guide" })).toHaveAttribute(
      "href",
      "https://bex.co/docs/cli",
    );
  });

  it("shows a switchable project / environment / service hierarchy", async () => {
    const user = userEvent.setup();
    const router = buildRouter(
      "/services/srv-api/settings",
      DashboardBreadcrumbs,
    );
    render(<RouterProvider router={router} />);

    expect(
      await screen.findByRole("navigation", { name: "Breadcrumbs" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Storefront/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Production/ }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /storefront-api/ }));
    expect(
      await screen.findByRole("menuitem", { name: /queue-worker/ }),
    ).toHaveAttribute("href", "/services/srv-worker");
    expect(
      screen.queryByRole("menuitem", { name: /billing-api/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "All resources" }),
    ).toHaveAttribute("href", "/");
  });

  it("renders the platform-neutral shortcut on the server, then the Mac glyph after mount", async () => {
    // Regression for the every-page React #418: on a non-Mac SSR host
    // navigator.platform is not "MacIntel", so reading it during render made
    // the Mac client's first paint ("⌘ K") disagree with the server ("Ctrl K")
    // — a hydration text mismatch on every page (this lives in the header).
    // Force a Mac platform and assert the SERVER render is still "Ctrl K" (so
    // the client's first render can match it), then that the client swaps to
    // "⌘ K" only after mount.
    const original = Object.getOwnPropertyDescriptor(navigator, "platform");
    Object.defineProperty(navigator, "platform", {
      value: "MacIntel",
      configurable: true,
    });
    try {
      const ssrRouter = buildRouter("/", GlobalSearch);
      await ssrRouter.load();
      const html = renderToString(<RouterProvider router={ssrRouter} />);
      expect(html).toContain("Ctrl K");
      expect(html).not.toContain("⌘");

      const router = buildRouter("/", GlobalSearch);
      render(<RouterProvider router={router} />);
      // After mount, platform detection upgrades the hint to the Mac glyph.
      await waitFor(() => expect(screen.getByText("⌘ K")).toBeInTheDocument());
    } finally {
      if (original) {
        Object.defineProperty(navigator, "platform", original);
      }
    }
  });

  it("opens workspace-wide search with the keyboard and navigates to a resource", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    expect(
      await screen.findByPlaceholderText("Search pages and resources…"),
    ).toBeInTheDocument();
    expect(screen.getByText("primary-db")).toBeInTheDocument();
    expect(screen.getByText("session-cache")).toBeInTheDocument();

    await user.click(screen.getByText("storefront-api"));
    expect(router.state.location.pathname).toBe("/services/srv-api");
  });

  // w6/m50: cmdk's bundled fuzzy scorer treated a subsequence match against a
  // resource's ~20-char id as relevant, so a short realistic query like "db"
  // returned most of the workspace instead of narrowing it. These pin the
  // fixed literal-substring behavior.
  it("narrows to only resources whose name/id/type contain the query, dropping unrelated ones (w6/m50)", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    await screen.findByPlaceholderText("Search pages and resources…");
    // None of storefront-api / queue-worker / billing-api / the projects /
    // key value / env group contain "db" — only the "primary-db" database does.
    await user.type(screen.getByRole("combobox"), "db");

    expect(await screen.findByText("primary-db")).toBeInTheDocument();
    expect(screen.queryByText("storefront-api")).not.toBeInTheDocument();
    expect(screen.queryByText("queue-worker")).not.toBeInTheDocument();
    expect(screen.queryByText("billing-api")).not.toBeInTheDocument();
    expect(screen.queryByText("Storefront")).not.toBeInTheDocument();
    expect(screen.queryByText("Billing")).not.toBeInTheDocument();
    expect(screen.queryByText("session-cache")).not.toBeInTheDocument();
  });

  it("still finds a resource by a raw id fragment (w6/m50)", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    await screen.findByPlaceholderText("Search pages and resources…");
    // "dpg-ma" only appears in the database's id (dpg-main), not its name
    // (primary-db) or type — a pure id-fragment paste.
    await user.type(screen.getByRole("combobox"), "dpg-ma");

    expect(await screen.findByText("primary-db")).toBeInTheDocument();
    expect(screen.queryByText("session-cache")).not.toBeInTheDocument();
  });

  // w6/046: with shouldFilter={false}, cmdk never prunes an itemless group —
  // the "Navigation" heading floated over zero children whenever a query
  // matched only resources (and "Resources" had the mirror-image artifact).
  it("hides the Navigation heading when a query matches only resources (w6/046)", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    await screen.findByPlaceholderText("Search pages and resources…");
    // Both headings render for the empty query…
    expect(screen.getByText("Navigation")).toBeInTheDocument();
    expect(screen.getByText("Resources")).toBeInTheDocument();
    expect(document.querySelector("[cmdk-separator]")).toBeInTheDocument();
    // …but "db" matches only the primary-db database, zero navigation pages.
    await user.type(screen.getByRole("combobox"), "db");

    expect(await screen.findByText("primary-db")).toBeInTheDocument();
    expect(screen.getByText("Resources")).toBeInTheDocument();
    expect(screen.queryByText("Navigation")).not.toBeInTheDocument();
    expect(document.querySelector("[cmdk-separator]")).toBeNull();
  });

  it("hides the Resources heading when a query matches only navigation pages (w6/046)", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    await screen.findByPlaceholderText("Search pages and resources…");
    // "webhooks" matches only the Webhooks nav page — no resource name, id,
    // or type-label contains it.
    await user.type(screen.getByRole("combobox"), "webhooks");

    expect(await screen.findByText("Webhooks")).toBeInTheDocument();
    expect(screen.getByText("Navigation")).toBeInTheDocument();
    expect(screen.queryByText("Resources")).not.toBeInTheDocument();
    expect(document.querySelector("[cmdk-separator]")).toBeNull();
  });

  it("shows the empty state when the query matches nothing (w6/m50)", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    await screen.findByPlaceholderText("Search pages and resources…");
    await user.type(screen.getByRole("combobox"), "zzznonexistentxyz999");

    expect(
      await screen.findByText("No matching pages or resources."),
    ).toBeInTheDocument();
  });
});
