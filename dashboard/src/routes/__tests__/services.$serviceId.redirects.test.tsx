import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceRedirectsPage } from "../services.$serviceId.redirects";
import { ServiceHeadersPage } from "../services.$serviceId.headers";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

const setRoutes = vi.fn(async () => true);
const setHeaders = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-static-site", () => ({
  useStaticSiteMutations: () => ({
    setRoutes,
    setHeaders,
    setPublishPath: vi.fn(async () => true),
    busy: false,
  }),
}));

function staticSvc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "srv-1",
    type: "static_site",
    routes: [{ type: "rewrite", source: "/*", destination: "/index.html" }],
    headers: [{ path: "/*", name: "X-Frame-Options", value: "DENY" }],
    ...overrides,
  } as ServiceView;
}

function renderPage(page: "redirects" | "headers") {
  const Component =
    page === "redirects" ? ServiceRedirectsPage : ServiceHeadersPage;
  const rootRoute = createRootRoute();
  const pageRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: `/services/$serviceId/${page}`,
    component: () => <Component serviceId="srv-1" />,
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/settings",
    component: () => <p>settings tab</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([pageRoute, settingsRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/srv-1/${page}`],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = staticSvc();
  serverState.loading = false;
  setRoutes.mockClear();
  setHeaders.mockClear();
});

describe("dedicated Redirects/Rewrites + Headers pages (w5/m48)", () => {
  it("renders the existing route rules and saves an edit through setRoutes", async () => {
    const user = userEvent.setup();
    renderPage("redirects");

    expect(await screen.findByText("Redirects/Rewrites")).toBeInTheDocument();
    expect(screen.getByDisplayValue("/index.html")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add rule" }));
    const sources = screen.getAllByPlaceholderText("/old/*");
    await user.type(sources[sources.length - 1], "/legacy/*");
    const destinations = screen.getAllByPlaceholderText("/index.html");
    await user.type(destinations[destinations.length - 1], "/new");
    await user.click(screen.getByRole("button", { name: "Save routes" }));

    expect(setRoutes).toHaveBeenCalledWith([
      { type: "rewrite", source: "/*", destination: "/index.html" },
      { type: "rewrite", source: "/legacy/*", destination: "/new" },
    ]);
  });

  it("renders the existing header rules and saves an edit through setHeaders", async () => {
    const user = userEvent.setup();
    renderPage("headers");

    expect(await screen.findByText("Headers")).toBeInTheDocument();
    expect(screen.getByDisplayValue("X-Frame-Options")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add header" }));
    const names = screen.getAllByPlaceholderText("X-Frame-Options");
    await user.type(names[names.length - 1], "Cache-Control");
    const values = screen.getAllByPlaceholderText("DENY");
    await user.type(values[values.length - 1], "no-store");
    await user.click(screen.getByRole("button", { name: "Save headers" }));

    expect(setHeaders).toHaveBeenCalledWith([
      { path: "/*", name: "X-Frame-Options", value: "DENY" },
      { path: "/*", name: "Cache-Control", value: "no-store" },
    ]);
  });

  it("redirects a non-static service to Settings (the pages only exist for static sites)", async () => {
    serverState.service = staticSvc({ type: "web_service" });
    renderPage("redirects");

    expect(await screen.findByText("settings tab")).toBeInTheDocument();
    expect(screen.queryByText("Redirects/Rewrites")).not.toBeInTheDocument();
  });
});
