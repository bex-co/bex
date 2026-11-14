import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  type RouteComponent,
} from "@tanstack/react-router";

// w4/641: every resource detail page makes its owning workspace the selection
// on entry, as the service pages do (w6/091). With the switcher on another of
// the caller's workspaces, opening the page selects the owner and keeps the
// skeleton up until it is the selection, so no control is judged by the wrong
// workspace's role. A non-member's selection never changes. The real
// useAdoptResourceWorkspace runs; only the workspace context, each page's data
// hook and the ready-state panels are stubbed.

const A = "tea-aaaaaaaaaaaaaaaaaaaa";
const B = "tea-bbbbbbbbbbbbbbbbbbbb";
const FOREIGN = "tea-cccccccccccccccccccc";

const setCurrentWorkspaceId = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [
      { id: A, name: "A" },
      { id: B, name: "B" },
    ],
    currentWorkspaceId: A,
    loading: false,
    setCurrentWorkspaceId,
  }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

const { stub, owner } = vi.hoisted(() => ({
  stub: () => null,
  owner: { id: "" },
}));

// --- Key Value ---
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: (id: string) => ({
    keyValue: {
      id,
      name: "kv-name",
      ownerId: owner.id,
      status: "available",
      plan: "free",
      version: null,
      createdAt: null,
      externalHost: null,
      public: false,
      suspended: false,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/components/key-value-metadata-card", () => ({
  KeyValueMetadataCard: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  KeyValueStatusBadge: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-danger-actions", () => ({
  KeyValueDangerActions: stub,
}));
vi.mock("@/features/keyvalue/components/connection-info-panel", () => ({
  ConnectionInfoPanel: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-networking-panel", () => ({
  KeyValueNetworkingPanel: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-plan-section", () => ({
  KeyValuePlanSection: stub,
}));
vi.mock(
  "@/features/keyvalue/components/key-value-maxmemory-policy-section",
  () => ({ KeyValueMaxmemoryPolicySection: stub }),
);
vi.mock(
  "@/features/keyvalue/components/key-value-persistence-mode-section",
  () => ({ KeyValuePersistenceModeSection: stub }),
);
vi.mock("@/features/keyvalue/components/key-value-detail-navigation", () => ({
  KeyValueDetailNavigation: stub,
}));

// --- Postgres ---
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: (id: string) => ({
    database: {
      id,
      name: "db-name",
      ownerId: owner.id,
      status: "available",
      plan: "free",
      version: "16",
      diskSizeGB: 1,
      createdAt: null,
      public: false,
      suspended: false,
      highAvailabilityEnabled: false,
      databaseName: "app",
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
vi.mock("@/features/databases/components/database-status-badge", () => ({
  DatabaseStatusBadge: stub,
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
vi.mock(
  "@/features/databases/components/database-disk-autoscaling-control",
  () => ({ DatabaseDiskAutoscalingControl: stub }),
);
vi.mock("@/common/components/deferred-mount", () => ({
  DeferredMount: () => null,
}));

// --- environment group ---
vi.mock(
  "@/features/env-groups/hooks/use-env-groups",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/env-groups/hooks/use-env-groups")
    >()),
    useEnvGroup: (id: string) => ({
      group: {
        id,
        name: "group-name",
        ownerId: owner.id,
        environmentId: null,
        createdAt: null,
        updatedAt: null,
        revision: 1,
        availability: null,
        serviceLinks: [],
        envVars: [],
        secretFiles: [],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
    useEnvGroupMutations: () => ({ busy: false }),
  }),
);
vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => {
  const scope = {
    projects: [],
    environments: [],
    byId: new Map(),
    serviceEnvironmentById: new Map(),
    loading: false,
    error: undefined,
  };
  return {
    useEnvGroupScopeIndex: () => scope,
    useWorkspaceEnvironmentIndex: () => scope,
  };
});
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [], loading: false, error: undefined }),
}));
vi.mock("@/features/env-groups/components/env-group-actions", () => ({
  EnvGroupActions: stub,
}));
vi.mock("@/features/env-groups/components/env-group-editors", () => ({
  EnvGroupEditors: stub,
}));
vi.mock("@/features/env-groups/components/env-group-metadata", () => ({
  EnvGroupMetadata: stub,
}));
vi.mock("@/features/env-groups/components/linked-services-card", () => ({
  LinkedServicesCard: stub,
}));

// --- Blueprint ---
vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: (id: string) => ({
    blueprint: {
      id,
      name: "blueprint-name",
      ownerId: owner.id,
      resourceSuffix: "",
      repo: "https://github.com/example/hello-go",
      branch: "main",
      path: "render.yaml",
      autoSync: true,
      manifest: "",
      status: "active",
      lastSync: null,
      resources: [],
      createdAt: null,
      updatedAt: null,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/blueprints/hooks/use-sync-blueprint", () => ({
  useSyncBlueprint: () => ({ sync: vi.fn(), busy: false }),
}));
vi.mock("@/features/blueprints/hooks/use-update-blueprint", () => ({
  useUpdateBlueprint: () => ({ update: vi.fn(), busy: false }),
}));
vi.mock("@/features/blueprints/hooks/use-disconnect-blueprint", () => ({
  useDisconnectBlueprint: () => ({ disconnect: vi.fn(), busy: false }),
}));
vi.mock("@/features/blueprints/hooks/use-blueprint-syncs", () => ({
  useBlueprintSyncs: () => ({ syncs: [], loading: false, error: undefined }),
}));
vi.mock("@/features/blueprints/hooks/use-planned-sync", () => ({
  usePlannedSync: () => ({
    planned: null,
    loading: false,
    busy: false,
    approve: vi.fn(),
    replan: vi.fn(),
  }),
}));
vi.mock("@/features/blueprints/hooks/use-blueprint-preview", () => ({
  useBlueprintPreview: () => ({
    preview: null,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/blueprints/components/validate-panel", () => ({
  ValidatePanel: stub,
}));

// --- webhook ---
vi.mock("@/features/webhooks/hooks/use-webhook", () => ({
  useWebhook: (id: string) => ({
    endpoint: {
      id,
      name: "webhook-name",
      ownerId: owner.id,
      url: "https://example.com/hook",
      eventTypes: [],
      enabled: true,
      disabledReason: "",
      secret: "",
      createdAt: null,
      createdBy: "",
    },
    loading: false,
    error: undefined,
    notFound: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/webhooks/hooks/use-webhook-creator", () => ({
  useWebhookCreator: () => null,
}));
vi.mock("@/features/webhooks/components/webhook-detail-header", () => ({
  WebhookDetailHeader: ({ endpoint }: { endpoint: { name: string } }) => (
    <h1>{endpoint.name}</h1>
  ),
}));

import { KeyValueDetailPage } from "../keyvalue.$keyValueId";
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { EnvGroupDetailPage } from "../env-groups_.$groupId";
import { BlueprintDetailPage } from "../blueprints.$blueprintId";
import { Route as WebhookRoute } from "../webhook.$webhookId";
import { Route as ProjectRoute } from "../project.$projectId";

interface PageCase {
  name: string;
  path: string;
  url: string;
  component: RouteComponent;
  /** What only the ready page shows. */
  ready: string;
  /** Layout routes: their child, which must not mount while adopting. */
  child?: boolean;
  loader?: () => unknown;
}

const PAGES: PageCase[] = [
  {
    name: "Key Value",
    path: "/keyvalue/$keyValueId",
    url: "/keyvalue/red-1",
    component: KeyValueDetailPage,
    ready: "kv-name",
  },
  {
    name: "Postgres",
    path: "/databases/$databaseId",
    url: "/databases/dpg-1",
    component: DatabaseRoute.options.component as RouteComponent,
    ready: "db-name",
  },
  {
    name: "environment group",
    path: "/env-groups/$groupId",
    url: "/env-groups/evg-1",
    component: EnvGroupDetailPage,
    ready: "group-name",
  },
  {
    name: "Blueprint",
    path: "/blueprints/$blueprintId",
    url: "/blueprints/exs-1",
    component: BlueprintDetailPage,
    ready: "blueprint-name",
  },
  {
    name: "webhook",
    path: "/webhook/$webhookId",
    url: "/webhook/whk-1",
    component: WebhookRoute.options.component as RouteComponent,
    ready: "webhook-name",
    child: true,
  },
  {
    name: "project",
    path: "/project/$projectId",
    url: "/project/prj-1",
    component: ProjectRoute.options.component as RouteComponent,
    ready: "child page",
    child: true,
    loader: () => ({
      state: "ready",
      resource: {
        id: "prj-1",
        name: "project-name",
        ownerId: owner.id,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    }),
  },
];

async function renderPage(page: PageCase) {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>home</div>,
  });
  // Mirrors the real route id so the page's own `Route` hooks resolve.
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: page.path,
    component: page.component,
    loader: page.loader,
  });
  const childRoute = createRoute({
    getParentRoute: () => detailRoute,
    path: "/",
    component: () => <div>child page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      homeRoute,
      page.child ? detailRoute.addChildren([childRoute]) : detailRoute,
    ]),
    history: createMemoryHistory({ initialEntries: [page.url] }),
  });
  render(<RouterProvider router={router} />);
  await router.load();
  return router;
}

beforeEach(() => {
  setCurrentWorkspaceId.mockReset();
});

describe.each(PAGES)("$name detail page (w4/641)", (page) => {
  it("selects the owning workspace and holds its skeleton until then", async () => {
    owner.id = B;
    await renderPage(page);
    await vi.waitFor(() =>
      expect(setCurrentWorkspaceId).toHaveBeenCalledExactlyOnceWith(B),
    );
    // The selection here is still A: nothing judged by A's role paints.
    expect(screen.queryByText(page.ready)).not.toBeInTheDocument();
  });

  it("leaves a non-member's selection alone and renders the page", async () => {
    owner.id = FOREIGN;
    await renderPage(page);
    expect(await screen.findAllByText(page.ready)).not.toHaveLength(0);
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });

  it("renders at once when the selection already owns it", async () => {
    owner.id = A;
    await renderPage(page);
    expect(await screen.findAllByText(page.ready)).not.toHaveLength(0);
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });
});
