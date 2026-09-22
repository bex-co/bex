import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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
import type {
  ServiceView,
  StaticRouteView,
  StaticHeaderView,
} from "@/features/services/types";
import type { StaticRuleSaveResult } from "@/features/services/hooks/use-static-site";
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

// The setters report the rows the server ACCEPTED, not just success — a padded
// path saves as its canonical form, so the editor has to be told what was
// stored or it stays dirty forever (w4/136). Each test sets the accepted rows.
const setRoutes = vi.fn(
  async (
    routes: StaticRouteView[],
  ): Promise<StaticRuleSaveResult<StaticRouteView>> => ({
    ok: true,
    saved: routes,
  }),
);
const setHeaders = vi.fn(
  async (
    headers: StaticHeaderView[],
  ): Promise<StaticRuleSaveResult<StaticHeaderView>> => ({
    ok: true,
    saved: headers,
  }),
);
vi.mock("@/features/services/hooks/use-static-site", () => ({
  useStaticSiteMutations: () => ({
    setRoutes,
    setHeaders,
    // Unchanged: the publish-directory sibling still reports a plain boolean.
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
  setRoutes.mockImplementation(async (routes) => ({ ok: true, saved: routes }));
  setHeaders.mockImplementation(async (headers) => ({
    ok: true,
    saved: headers,
  }));
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

// w4/136: the backend trims route source/destination and header path, so a
// padded path saves successfully as its canonical form. The editors seeded
// their draft on mount and compared it to the refetched props, so after an
// accepted normalized save the draft still held the padded text — permanently
// dirty, Save enabled and Cancel present, for a change that had already
// persisted. Live, both editors reproduced this twice.
describe("post-save synchronization with the accepted rows (w4/136)", () => {
  it("adopts the canonical route rows and returns to a clean state", async () => {
    const user = userEvent.setup();
    // The server accepts the save and stores the trimmed form.
    setRoutes.mockImplementation(async (routes) => {
      const saved = routes.map((route) => ({
        ...route,
        source: route.source.trim(),
        destination: route.destination.trim(),
      }));
      serverState.service = staticSvc({ routes: saved });
      return { ok: true, saved };
    });
    renderPage("redirects");
    await screen.findByText("Redirects/Rewrites");

    const source = screen.getByDisplayValue("/*");
    await user.clear(source);
    await user.type(source, " /qa-alias/* ");
    const save = screen.getByRole("button", { name: "Save routes" });
    expect(save).toBeEnabled();

    await user.click(save);

    // Read .value exactly: ByDisplayValue normalizes whitespace, so it cannot
    // tell " /qa-alias/* " from "/qa-alias/*" — which is the entire defect.
    // (The Save/Cancel clean state also needs the refetched PROPS to arrive,
    // which this page-level harness mocks away; static-site-section.test.tsx
    // asserts that half with controlled props.)
    await waitFor(() =>
      expect(
        (screen.getByRole("textbox", { name: "Source" }) as HTMLInputElement)
          .value,
      ).toBe("/qa-alias/*"),
    );
  });

  it("adopts the canonical header rows and returns to a clean state", async () => {
    const user = userEvent.setup();
    setHeaders.mockImplementation(async (headers) => {
      const saved = headers.map((header) => ({
        ...header,
        path: header.path.trim(),
        name: header.name.trim(),
        // Header VALUES are preserved byte for byte by the backend.
        value: header.value,
      }));
      serverState.service = staticSvc({ headers: saved });
      return { ok: true, saved };
    });
    renderPage("headers");
    await screen.findByText("Headers");

    const path = screen.getByDisplayValue("/*");
    await user.clear(path);
    await user.type(path, " /qa/* ");
    await user.click(screen.getByRole("button", { name: "Save headers" }));

    await waitFor(() =>
      expect(
        (screen.getByRole("textbox", { name: "Path" }) as HTMLInputElement)
          .value,
      ).toBe("/qa/*"),
    );
    // The value was never trimmed, in the editor or on the wire.
    expect(screen.getByRole("textbox", { name: "Value" })).toHaveValue("DENY");
  });

  it("keeps the draft and stays dirty when the save is refused", async () => {
    const user = userEvent.setup();
    setRoutes.mockImplementation(async () => ({ ok: false }));
    renderPage("redirects");
    await screen.findByText("Redirects/Rewrites");

    const source = screen.getByDisplayValue("/*");
    await user.clear(source);
    await user.type(source, "/kept/*");
    await user.click(screen.getByRole("button", { name: "Save routes" }));

    expect(screen.getByDisplayValue("/kept/*")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save routes" })).toBeEnabled();
  });

  it("does not empty the editor when the post-save read produces no service", async () => {
    const user = userEvent.setup();
    // ok, but `saved` absent — an unreadable list must not read as "no rules".
    setRoutes.mockImplementation(async () => ({ ok: true }));
    renderPage("redirects");
    await screen.findByText("Redirects/Rewrites");

    const source = screen.getByDisplayValue("/*");
    await user.clear(source);
    await user.type(source, "/still-here/*");
    await user.click(screen.getByRole("button", { name: "Save routes" }));

    expect(screen.getByDisplayValue("/still-here/*")).toBeInTheDocument();
  });

  it("preserves a newer draft typed while the save was in flight", async () => {
    const user = userEvent.setup();
    let release: () => void = () => {};
    setRoutes.mockImplementation(async (routes) => {
      await new Promise<void>((resolve) => {
        release = resolve;
      });
      return {
        ok: true,
        saved: routes.map((route) => ({
          ...route,
          source: route.source.trim(),
        })),
      };
    });
    renderPage("redirects");
    await screen.findByText("Redirects/Rewrites");

    const source = screen.getByDisplayValue("/*");
    await user.clear(source);
    await user.type(source, " /first/* ");
    await user.click(screen.getByRole("button", { name: "Save routes" }));

    // Rows stay editable while the request is in flight, so the user types on.
    const inFlight = screen.getByRole("textbox", { name: "Source" });
    await user.clear(inFlight);
    await user.type(inFlight, "/second/*");

    release();
    await screen.findByDisplayValue("/second/*");

    // The in-flight save's accepted rows must NOT overwrite the newer draft.
    expect(screen.queryByDisplayValue("/first/*")).toBeNull();
    expect(screen.queryByDisplayValue(" /first/* ")).toBeNull();
  });
});
