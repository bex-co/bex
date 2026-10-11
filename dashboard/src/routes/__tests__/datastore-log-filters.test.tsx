import { beforeEach, describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";
import {
  act,
  cleanup,
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
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// w4/228: the datastore Logs tab's text and instance filters lived in local
// state, so a reload or a copied link silently dropped them while the range
// survived. Both production mounts are exercised through their real route
// components and validators.

const queries = vi.hoisted(
  () => [] as { resource: string; text: string; instance: string }[],
);
const labels = vi.hoisted(() => ({ values: [] as string[] }));

vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: (
    resource: string,
    filters: { text: string; instance: string },
  ) => {
    queries.push({ resource, text: filters.text, instance: filters.instance });
    return {
      lines: [],
      loading: false,
      error: undefined,
      storeUnavailable: false,
      timedOut: false,
      hasMore: false,
      loadingOlder: false,
      loadOlder: () => undefined,
    };
  },
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelValues: () => labels.values,
}));
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: () => ({
    database: {
      id: "dpg-main",
      name: "primary-db",
      status: "available",
      suspended: false,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/databases/hooks/use-database-lifecycle", () => ({
  useDatabaseLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: () => ({
    keyValue: {
      id: "red-cache",
      name: "session-cache",
      status: "available",
      plan: "free",
      version: "8",
      createdAt: null,
      externalHost: null,
      public: false,
      suspended: false,
      region: null,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-lifecycle", () => ({
  useKeyValueLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: [], loading: false, error: undefined }),
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [],
    loading: false,
    error: undefined,
  }),
}));

setupVirtualGeometry();

const MOUNTS = [
  {
    name: "Postgres",
    route: DatabaseRoute,
    path: "/databases/$databaseId",
    base: "/databases/dpg-main",
    resource: "dpg-main",
    instance: "dpg-main-0",
    instanceLabel: "Database instance",
  },
  {
    name: "Key Value",
    route: KeyValueRoute,
    path: "/keyvalue/$keyValueId",
    base: "/keyvalue/red-cache",
    resource: "red-cache",
    instance: "red-cache-931irt8468joti9ne4fj",
    instanceLabel: "Instance",
  },
] as const;

type Mount = (typeof MOUNTS)[number];

function renderAt(mount: Mount, href: string) {
  const root = createRootRoute();
  const page = createRoute({
    getParentRoute: () => root,
    path: mount.path,
    validateSearch: mount.route.options.validateSearch,
    component: mount.route.options.component,
  });
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const searchBox = () => screen.findByRole("textbox");

beforeEach(() => {
  queries.length = 0;
  labels.values = [];
});

describe.each(MOUNTS)("$name Logs tab filter URL state (w4/228)", (mount) => {
  const instancePicker = () =>
    screen.getByRole("combobox", { name: mount.instanceLabel });

  it("restores text + instance from a copied link, querying them first, without rewriting the URL", async () => {
    const href = `${mount.base}?tab=logs&range=7d&text=Ready%20to%20accept&instance=${mount.instance}`;
    const router = renderAt(mount, href);

    expect(await searchBox()).toHaveValue("Ready to accept");
    // Discovery has not named the instance yet; the linked one still shows.
    expect(instancePicker()).toHaveTextContent(mount.instance);
    // The very first history query is the filtered one — no unfiltered flash.
    expect(queries[0]).toEqual({
      resource: mount.resource,
      text: "Ready to accept",
      instance: mount.instance,
    });
    expect(queries.every((q) => q.text === "Ready to accept")).toBe(true);
    expect(router.state.location.search).toMatchObject({
      tab: "logs",
      range: "7d",
      text: "Ready to accept",
      instance: mount.instance,
    });
    expect(router.history.length).toBe(1);
  });

  it("writes typed text and a picked instance, and survives a reload", async () => {
    labels.values = [mount.instance];
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const router = renderAt(mount, `${mount.base}?tab=logs&range=7d`);

    fireEvent.change(await searchBox(), { target: { value: "checkpoint" } });
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({
        text: "checkpoint",
      }),
    );
    await user.click(instancePicker());
    await user.click(screen.getByRole("option", { name: mount.instance }));
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({
        tab: "logs",
        range: "7d",
        text: "checkpoint",
        instance: mount.instance,
      }),
    );

    // Reload: a fresh router at the written URL restores both controls.
    const href = router.state.location.href;
    cleanup();
    queries.length = 0;
    labels.values = [];
    renderAt(mount, href);
    expect(await searchBox()).toHaveValue("checkpoint");
    expect(instancePicker()).toHaveTextContent(mount.instance);
    expect(queries[0]).toMatchObject({
      text: "checkpoint",
      instance: mount.instance,
    });
  });

  it("clearing a filter removes only its key", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const router = renderAt(
      mount,
      `${mount.base}?tab=logs&range=custom&rangeStart=2026-10-03T07:00:00.000Z&rangeEnd=2026-10-03T08:00:00.000Z&text=x&instance=${mount.instance}`,
    );

    fireEvent.change(await searchBox(), { target: { value: "" } });
    await user.click(instancePicker());
    await user.click(screen.getByRole("option", { name: /All instances/ }));
    await waitFor(() => {
      const search = router.state.location.search as Record<string, unknown>;
      expect(search.text).toBeUndefined();
      expect(search.instance).toBeUndefined();
    });
    expect(router.state.location.searchStr).not.toMatch(/text|instance/);
    expect(router.state.location.search).toMatchObject({
      tab: "logs",
      range: "custom",
      rangeStart: "2026-10-03T07:00:00.000Z",
      rangeEnd: "2026-10-03T08:00:00.000Z",
    });
  });

  it("round-trips numeric-looking text as a string", async () => {
    const router = renderAt(mount, `${mount.base}?tab=logs&text=404`);
    expect(await searchBox()).toHaveValue("404");
    expect(queries[0]).toMatchObject({ text: "404" });

    fireEvent.change(await searchBox(), { target: { value: "500" } });
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ text: "500" }),
    );
    const href = router.state.location.href;
    cleanup();
    queries.length = 0;
    renderAt(mount, href);
    expect(await searchBox()).toHaveValue("500");
    expect(queries[0]).toMatchObject({ text: "500" });
  });

  it("adopts a URL change it did not write (back/forward)", async () => {
    const router = renderAt(mount, `${mount.base}?tab=logs&text=first`);
    expect(await searchBox()).toHaveValue("first");

    await act(async () => {
      await router.navigate({
        to: ".",
        search: (prev: Record<string, unknown>) => ({
          ...prev,
          text: "second",
          instance: mount.instance,
        }),
      });
    });
    await waitFor(() =>
      expect(screen.getByRole("textbox")).toHaveValue("second"),
    );
    expect(instancePicker()).toHaveTextContent(mount.instance);
    expect(queries.at(-1)).toMatchObject({
      text: "second",
      instance: mount.instance,
    });

    queries.length = 0;
    await act(async () => {
      router.history.back();
    });
    await waitFor(() =>
      expect(screen.getByRole("textbox")).toHaveValue("first"),
    );
    expect(instancePicker()).toHaveTextContent("All instances");
    // Adopted straight away — never re-querying the value being left.
    expect(queries.at(-1)).toMatchObject({ text: "first", instance: "" });
    expect(queries.some((q) => q.text === "second" && q.instance === "")).toBe(
      false,
    );
    expect(router.state.location.search).toMatchObject({ text: "first" });
  });

  it("Back within the debounce drops the typed text rather than re-writing it", async () => {
    const router = renderAt(mount, `${mount.base}?tab=logs&text=first`);
    expect(await searchBox()).toHaveValue("first");
    await act(async () => {
      await router.navigate({
        to: ".",
        search: (prev: Record<string, unknown>) => ({
          ...prev,
          text: "second",
        }),
      });
    });
    await waitFor(() =>
      expect(screen.getByRole("textbox")).toHaveValue("second"),
    );

    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "typed" },
    });
    await act(async () => {
      router.history.back();
    });
    await waitFor(() =>
      expect(screen.getByRole("textbox")).toHaveValue("first"),
    );
    await act(() => new Promise((resolve) => setTimeout(resolve, 400)));
    expect(screen.getByRole("textbox")).toHaveValue("first");
    expect(router.state.location.search).toMatchObject({ text: "first" });
    expect(queries.some((q) => q.text === "typed")).toBe(false);
  });
});
