import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";
import { codedGraphQLError } from "@/test/mocks/apollo";

// w3/m706: every resource page keeps a dead link's URL and draws Render's
// state in place — never a redirect home and never a toast (w9/m55's). Each
// page's data hook answers an empty read with the case's error; the route's
// loader answers with the case's state, as the real loader would.

const read = vi.hoisted(() => ({
  error: undefined as Error | undefined,
  routeState: "not-found" as string,
}));
const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({
  toast: { error: toastError, success: vi.fn() },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [{ id: "tea-mine", name: "Mine" }],
    currentWorkspaceId: "tea-mine",
    loading: false,
    setCurrentWorkspaceId: vi.fn(),
  }),
}));
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
const empty = () => ({
  loading: false,
  get error() {
    return read.error;
  },
  refetch: vi.fn(),
});
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: () => ({ database: null, ...empty() }),
}));
vi.mock("@/features/databases/hooks/use-database-lifecycle", () => ({
  useDatabaseLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: () => ({ keyValue: null, ...empty() }),
}));
vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: () => ({ blueprint: null, ...empty() }),
}));
vi.mock("@/features/webhooks/hooks/use-webhook", () => ({
  useWebhook: () => ({ endpoint: null, notFound: false, ...empty() }),
}));
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: () => null,
}));
vi.mock("@/features/sandbox-groups/hooks/use-sandbox-groups", () => ({
  useSandboxGroup: () => ({ group: null, ...empty() }),
}));
const missingGroup = vi.hoisted(() => ({
  name: null as string | null,
  projectId: null as string | null,
}));
vi.mock("@/common/hooks/use-missing-resource-context", () => ({
  useMissingSandboxGroupContext: () => missingGroup,
}));

import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { Route as BlueprintRoute } from "../blueprints.$blueprintId";
import { Route as WebhookRoute } from "../webhook.$webhookId";
import { Route as ProjectRoute } from "../project.$projectId";
import { Route as SandboxGroupRoute } from "../sandbox-groups_.$sandboxGroupId";

interface PageCase {
  name: string;
  path: string;
  id: string;
  url: string;
  component: RouteComponent;
  /** The page's own not-found title (Render's, per page). */
  notFound: string;
  /** The button Page not found and Access denied draw (Go to Webhooks). */
  action?: [string, string];
}

const PAGES: PageCase[] = [
  {
    name: "Postgres",
    path: "/databases/$databaseId",
    id: "dpg-gone",
    url: "/databases/dpg-gone/logs",
    component: DatabaseRoute.options.component as RouteComponent,
    notFound: "Page not found",
  },
  {
    name: "Key Value",
    path: "/keyvalue/$keyValueId",
    id: "red-gone",
    url: "/keyvalue/red-gone",
    component: KeyValueRoute.options.component as RouteComponent,
    notFound: "Page not found",
  },
  {
    name: "Blueprint",
    path: "/blueprints/$blueprintId",
    id: "exs-gone",
    url: "/blueprints/exs-gone",
    component: BlueprintRoute.options.component as RouteComponent,
    notFound: "Page not found",
  },
  {
    name: "webhook",
    path: "/webhook/$webhookId",
    id: "whk-gone",
    url: "/webhook/whk-gone",
    component: WebhookRoute.options.component as RouteComponent,
    notFound: "Page not found",
    action: ["Go to Webhooks", "/webhooks"],
  },
  {
    name: "project",
    path: "/project/$projectId",
    id: "prj-gone",
    url: "/project/prj-gone",
    component: ProjectRoute.options.component as RouteComponent,
    notFound: "Page not found",
  },
  {
    name: "sandbox group",
    path: "/sandbox-groups_/$sandboxGroupId",
    id: "sbg-gone",
    url: "/sandbox-groups/sbg-gone",
    component: SandboxGroupRoute.options.component as RouteComponent,
    notFound: "Sandbox group not found",
  },
];

async function renderPage(page: PageCase) {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>home page</div>,
  });
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/webhooks",
    component: () => <div>webhooks page</div>,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: page.path.replace("sandbox-groups_", "sandbox-groups"),
    component: page.component,
    loader: () => ({ state: read.routeState }),
  });
  const childRoute = createRoute({
    getParentRoute: () => detailRoute,
    path: "$",
    component: () => <div>child page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      homeRoute,
      listRoute,
      detailRoute.addChildren([childRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [page.url] }),
  });
  render(<RouterProvider router={router} />);
  await router.load();
  return router;
}

function stateRegion() {
  const region = document.querySelector<HTMLElement>("[data-resource-state]");
  if (!region) throw new Error("no state drawn");
  return within(region);
}

beforeEach(() => {
  read.error = undefined;
  read.routeState = "not-found";
  missingGroup.name = null;
  missingGroup.projectId = null;
  toastError.mockReset();
});

describe.each(PAGES)("$name dead link (w3/m706)", (page) => {
  it("draws its not-found state in place: the URL stays and nothing toasts", async () => {
    read.error = codedGraphQLError("NOT_FOUND", {}, "not found");
    const router = await renderPage(page);

    expect(
      await screen.findByRole("heading", { name: page.notFound }),
    ).toBeInTheDocument();
    const state = stateRegion();
    expect(state.getByText(page.id)).toBeInTheDocument();
    const [label, href] = page.action ?? ["Return home", "/"];
    expect(state.getAllByRole("link")[0]).toHaveTextContent(label);
    expect(state.getAllByRole("link")[0]).toHaveAttribute("href", href);
    expect(router.state.location.pathname).toBe(page.url);
    expect(screen.queryByText("child page")).not.toBeInTheDocument();
    expect(screen.queryByText("home page")).not.toBeInTheDocument();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("draws Access denied for a refused read", async () => {
    read.error = codedGraphQLError("FORBIDDEN", {}, "forbidden");
    read.routeState = "forbidden";
    const router = await renderPage(page);

    expect(
      await screen.findByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
    expect(stateRegion().getByText(page.id)).toBeInTheDocument();
    if (page.action) {
      expect(
        stateRegion().getByRole("link", { name: page.action[0] }),
      ).toHaveAttribute("href", page.action[1]);
    }
    expect(router.state.location.pathname).toBe(page.url);
  });

  // Apollo caches a refused read's null without its error: the loader that
  // saw the refusal decides.
  it("draws Access denied when only the route's loader saw the refusal", async () => {
    read.routeState = "forbidden";
    await renderPage(page);

    expect(
      await screen.findByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
  });

  it("draws An error occurred for any other failure", async () => {
    read.error = new Error("Failed to fetch");
    read.routeState = "error";
    const router = await renderPage(page);

    expect(
      await screen.findByRole("heading", { name: "An error occurred" }),
    ).toBeInTheDocument();
    expect(
      stateRegion().getByText(/could not be loaded\./),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(page.url);
  });
});

// Render's sandbox group reads its cached name and project (its
// missingSandboxGroupContext fragment).
it("a sandbox group the dashboard had read reads deleted, with Go to project", async () => {
  read.error = codedGraphQLError("NOT_FOUND", {}, "not found");
  missingGroup.name = "agents";
  missingGroup.projectId = "prj-shop";
  await renderPage(PAGES.find((page) => page.name === "sandbox group")!);

  expect(
    await screen.findByRole("heading", { name: "Sandbox group deleted" }),
  ).toBeInTheDocument();
  const state = stateRegion();
  expect(state.getByText("agents")).toBeInTheDocument();
  expect(state.getAllByRole("link").map((link) => link.textContent)).toEqual([
    "Go to project",
    "Return home",
  ]);
});
