import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ProjectPage } from "../project.$projectId.index";
import { ProjectResourceFilters } from "@/features/projects/components/project-resource-filters";
import {
  parseProjectResourceSearch,
  type ProjectResourceFilterState,
} from "@/features/projects/lib/resource-filter";

// w4/266: the project filter rendered only from Route.useSearch(), which the
// router publishes inside a transition. A keystroke's controlled value was
// rolled back before the URL caught up ("second" became "econd"), and the
// next keystroke spread the stale kind back over a just-clicked type. These
// drive the real ProjectPage, router and ProjectResourceFilters input; the
// Apollo-backed EnvironmentsPanel is reduced to the filter it forwards.
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [], loading: false, refetch: vi.fn() }),
}));
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ databases: [], loading: false, refetch: vi.fn() }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ keyValues: [], loading: false, refetch: vi.fn() }),
}));
vi.mock("@/features/projects/hooks/use-rename-project", () => ({
  useRenameProject: () => ({ rename: vi.fn(), busy: false }),
}));
vi.mock("@/features/environments/components/environments-panel", () => ({
  EnvironmentsPanel: ({
    resourceFilter,
    onResourceFilterChange,
  }: {
    resourceFilter: ProjectResourceFilterState;
    onResourceFilterChange: (next: ProjectResourceFilterState) => void;
  }) => (
    <div>
      <ProjectResourceFilters
        environmentName="first"
        filter={resourceFilter}
        counts={{
          all: 2,
          services: 0,
          databases: 0,
          keyvalues: 0,
          envgroups: 2,
        }}
        onChange={onResourceFilterChange}
      />
      <button
        type="button"
        onClick={() =>
          onResourceFilterChange({
            environmentId: "evm-second",
            query: "",
            kind: "all",
          })
        }
      >
        Switch environment
      </button>
    </div>
  ),
}));

const PROJECT = {
  id: "prj-1",
  name: "filters",
  ownerId: "tea-1",
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
};

function renderPage(initial: string) {
  const rootRoute = createRootRoute();
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    loader: () => ({ state: "ready", resource: PROJECT }) as const,
  });
  const indexRoute = createRoute({
    getParentRoute: () => projectRoute,
    path: "/",
    component: ProjectPage,
    validateSearch: parseProjectResourceSearch,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([projectRoute.addChildren([indexRoute])]),
    history: createMemoryHistory({ initialEntries: [initial] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

function urlSearch(router: ReturnType<typeof renderPage>) {
  return router.state.location.search as Record<string, unknown>;
}

/** Types like a user: each keystroke appends to whatever the DOM holds now,
 *  without waiting for the router between characters. */
function typeFast(input: HTMLElement, text: string) {
  for (const char of text) {
    fireEvent.change(input, {
      target: { value: (input as HTMLInputElement).value + char },
    });
  }
}

const START = "/project/prj-1?env=evm-first&kind=services";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("project resource filter state", () => {
  it("keeps every typed character and the just-clicked kind", async () => {
    const router = renderPage(START);
    const input = await screen.findByRole("searchbox");
    const envGroups = screen.getByRole("button", { name: /Env Groups/ });

    fireEvent.click(envGroups);
    typeFast(input, "second");

    expect(input).toHaveValue("second");
    expect(envGroups.className).toMatch(/bg-primary/);
    await waitFor(() =>
      expect(urlSearch(router)).toMatchObject({
        env: "evm-first",
        q: "second",
        kind: "envgroups",
      }),
    );
    expect(input).toHaveValue("second");
    expect(
      screen.getByRole("button", { name: /Env Groups/ }).className,
    ).toMatch(/bg-primary/);
  });

  it("keeps the first character on a fresh page with the kind already set", async () => {
    const router = renderPage("/project/prj-1?env=evm-first&kind=envgroups");
    const input = await screen.findByRole("searchbox");

    typeFast(input, "second");

    expect(input).toHaveValue("second");
    await waitFor(() => expect(urlSearch(router).q).toBe("second"));
    expect(input).toHaveValue("second");
  });

  it("still works when typing waits for each URL update (warm control)", async () => {
    const router = renderPage(START);
    const input = await screen.findByRole("searchbox");
    fireEvent.click(screen.getByRole("button", { name: /Env Groups/ }));
    await waitFor(() => expect(urlSearch(router).kind).toBe("envgroups"));
    for (const char of "second") {
      const value = (input as HTMLInputElement).value + char;
      fireEvent.change(input, { target: { value } });
      await waitFor(() => expect(urlSearch(router).q).toBe(value));
    }
    expect(input).toHaveValue("second");
  });

  it("applies an environment switch's reset synchronously", async () => {
    const router = renderPage(
      "/project/prj-1?env=evm-first&q=sec&kind=envgroups",
    );
    const input = await screen.findByRole("searchbox");
    expect(input).toHaveValue("sec");

    fireEvent.click(screen.getByRole("button", { name: "Switch environment" }));

    expect(input).toHaveValue("");
    expect(screen.getByRole("button", { name: /All/ }).className).toMatch(
      /bg-primary/,
    );
    await waitFor(() =>
      expect(urlSearch(router)).toMatchObject({ env: "evm-second" }),
    );
    expect(urlSearch(router).q).toBeUndefined();
  });

  it("lets an external navigation replace a pending draft", async () => {
    const router = renderPage(START);
    const input = await screen.findByRole("searchbox");
    // No wait: the draft is still ahead of the URL when the external
    // navigation lands.
    typeFast(input, "sec");
    expect(input).toHaveValue("sec");

    await act(() =>
      router.navigate({
        to: "/project/$projectId",
        params: { projectId: "prj-1" },
        search: { env: "evm-first", q: "shared", kind: "databases" },
      }),
    );

    await waitFor(() => expect(urlSearch(router).q).toBe("shared"));
    expect(input).toHaveValue("shared");
    expect(screen.getByRole("button", { name: /Databases/ }).className).toMatch(
      /bg-primary/,
    );
  });
});
