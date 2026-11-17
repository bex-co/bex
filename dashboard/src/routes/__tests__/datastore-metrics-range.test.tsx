import { beforeEach, describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { Route as DiskRoute } from "../services.$serviceId.disk";
import type { RangeSelection } from "@/features/metrics/lib/range";

// w4/703: the datastore Metrics range (Key Value tab, Postgres overview,
// service Disk tab) lived in the panel's own state, so a reload or a copied
// link dropped it while service Metrics and the datastore Logs tab kept theirs
// in the URL (w6/065). Each production mount runs through its real route
// component and validator; the panel is a stub that shows the range it was
// handed and offers the picks, since its own suite covers the picker.

const CUSTOM = {
  start: "2026-07-01T00:00:00.000Z",
  end: "2026-07-01T06:00:00.000Z",
};

vi.mock("@/features/metrics/components/datastore-metrics-panel", async () => {
  const { RANGE_PRESETS, makeCustomRange } = await import(
    "@/features/metrics/lib/range"
  );
  return {
    DatastoreMetricsPanel: ({
      kind,
      range,
      onRangeChange,
    }: {
      kind: string;
      range?: RangeSelection;
      onRangeChange?: (range: RangeSelection) => void;
    }) => (
      <div>
        <p data-testid="metrics-range">
          {kind}:
          {range == null
            ? "uncontrolled"
            : "startTime" in range
              ? `custom ${range.startTime}..${range.endTime}`
              : range.id}
        </p>
        <button onClick={() => onRangeChange?.(RANGE_PRESETS[2]!)}>
          pick 4h
        </button>
        <button
          onClick={() =>
            onRangeChange?.(
              makeCustomRange(CUSTOM.start, CUSTOM.end) as RangeSelection,
            )
          }
        >
          pick custom
        </button>
      </div>
    ),
  };
});

// Only the metrics section mounts on the Postgres overview; the other deferred
// panels are Apollo reads this route test does not provide.
vi.mock("@/common/components/deferred-mount", () => ({
  DeferredMount: ({
    hashId,
    children,
  }: {
    hashId?: string;
    children: ReactNode;
  }) => (hashId === "metrics" ? <>{children}</> : null),
}));

const stub = () => null;
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: () => ({
    database: {
      id: "dpg-main",
      name: "primary-db",
      status: "available",
      plan: "free",
      suspended: false,
      highAvailabilityEnabled: false,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/databases/hooks/use-database-lifecycle", () => ({
  useDatabaseLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/components/database-metadata-card", () => ({
  DatabaseMetadataCard: stub,
}));
vi.mock("@/features/databases/components/database-row-actions", () => ({
  DatabaseRowActions: stub,
}));
vi.mock("@/features/databases/components/database-danger-actions", () => ({
  DatabaseDangerActions: stub,
}));
vi.mock("@/features/databases/components/connection-info-panel", () => ({
  ConnectionInfoPanel: stub,
}));
vi.mock("@/features/databases/components/database-detail-navigation", () => ({
  DatabaseDetailNavigation: stub,
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
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { id: "srv-disk", type: "web_service", plan: "starter" },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/services/hooks/use-disks", () => ({
  useDisk: () => ({
    disk: {
      id: "dsk-1",
      name: "data",
      mountPath: "/var/data",
      sizeGB: 10,
      serviceId: "srv-disk",
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
  useDiskSnapshots: () => ({
    snapshots: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
  useDiskSnapshotWarning: () => null,
  useDiskMutations: () => ({
    addDisk: vi.fn(),
    growDisk: vi.fn(),
    deleteDisk: vi.fn(),
    restoreSnapshot: vi.fn(),
    addError: null,
    clearAddError: vi.fn(),
    busy: false,
  }),
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

const MOUNTS = [
  {
    name: "Key Value Metrics tab",
    route: KeyValueRoute,
    path: "/keyvalue/$keyValueId",
    base: "/keyvalue/red-cache?tab=metrics",
    kind: "keyvalue",
    keeps: { tab: "metrics" },
  },
  {
    name: "Postgres overview metrics",
    route: DatabaseRoute,
    path: "/databases/$databaseId",
    base: "/databases/dpg-main?",
    kind: "database",
    keeps: {},
  },
  {
    name: "service Disk tab",
    route: DiskRoute,
    path: "/services/$serviceId/disk",
    base: "/services/srv-disk/disk?",
    kind: "service",
    keeps: {},
  },
] as const;

type Mount = (typeof MOUNTS)[number];

async function renderAt(mount: Mount, href: string) {
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
  await router.load();
  render(<RouterProvider router={router} />);
  return router;
}

const shownRange = () => screen.findByTestId("metrics-range");
const join = (mount: Mount, query: string) =>
  mount.base.endsWith("?") ? `${mount.base}${query}` : `${mount.base}&${query}`;

beforeEach(() => {
  cleanup();
});

describe.each(MOUNTS)("$name range URL state (w4/703)", (mount) => {
  it("opens on Render's 12-hour default when the URL names no range", async () => {
    await renderAt(mount, mount.base);
    expect(await shownRange()).toHaveTextContent(`${mount.kind}:12h`);
  });

  it("opens a copied ?range=4h link on that preset without rewriting the URL", async () => {
    const router = await renderAt(mount, join(mount, "range=4h"));
    expect(await shownRange()).toHaveTextContent(`${mount.kind}:4h`);
    expect(router.history.length).toBe(1);
  });

  it("writes a pick to the URL, keeps it across a reload, and clears custom bounds on a preset", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const router = await renderAt(mount, mount.base);

    await user.click(await screen.findByRole("button", { name: "pick custom" }));
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        ...mount.keeps,
        range: "custom",
        rangeStart: CUSTOM.start,
        rangeEnd: CUSTOM.end,
      }),
    );
    expect(await shownRange()).toHaveTextContent(
      `${mount.kind}:custom ${CUSTOM.start}..${CUSTOM.end}`,
    );

    // Reload: a fresh router at the written URL restores the custom window.
    const customHref = router.state.location.href;
    cleanup();
    const reloaded = await renderAt(mount, customHref);
    expect(await shownRange()).toHaveTextContent(
      `${mount.kind}:custom ${CUSTOM.start}..${CUSTOM.end}`,
    );

    // A preset pick replaces the custom bounds instead of leaving them stale.
    await user.click(screen.getByRole("button", { name: "pick 4h" }));
    await waitFor(() =>
      expect(reloaded.state.location.search).toEqual({
        ...mount.keeps,
        range: "4h",
      }),
    );
    expect(await shownRange()).toHaveTextContent(`${mount.kind}:4h`);

    const presetHref = reloaded.state.location.href;
    cleanup();
    await renderAt(mount, presetHref);
    expect(await shownRange()).toHaveTextContent(`${mount.kind}:4h`);
  });
});
